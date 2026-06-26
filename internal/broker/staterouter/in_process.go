package staterouter

import (
	"context"
	"errors"
	"fmt"

	"github.com/BAN1ce/skyTree/internal/broker/delivery"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/session"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/subscription"
	"github.com/BAN1ce/skyTree/pkg/cluster"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/proto/proto_session"
	"github.com/BAN1ce/skyTree/proto/proto_topic"
)

// PublishRouteHandler routes a client-published message to its subscribers.
// It is injected so the state router does not depend on the delivery layer directly.
type PublishRouteHandler func(context.Context, RoutePublishRequest) error

// InProcessDependencies bundles the collaborators required to build an
// InProcessClient. SessionCenter, SubscriptionCenter and PublishRoute are
// mandatory; NodeController is only needed for cross-node owner takeover and
// may be nil on a single-node deployment.
type InProcessDependencies struct {
	SessionCenter      session.Center
	SubscriptionCenter subscription.Center
	NodeController     cluster.NodeController
	PublishRoute       PublishRouteHandler
}

// InProcessClient is the in-process implementation of the Client boundary.
// It serves state and route operations by calling the local session and
// subscription centers directly (no network hop). The struct holds no
// per-connection state: caller identity (broker node, client id, owner token)
// travels in every request, so a single instance is shared by all connections
// and the same signatures can back a future remote/RPC implementation.
type InProcessClient struct {
	sessionCenter      session.Center
	subscriptionCenter subscription.Center
	nodeController     cluster.NodeController
	publishRoute       PublishRouteHandler
}

// NewInProcessClient validates the mandatory dependencies and returns a ready
// InProcessClient. It returns an error (rather than panicking) when a required
// collaborator is missing so wiring mistakes surface at startup.
func NewInProcessClient(deps InProcessDependencies) (*InProcessClient, error) {
	if deps.SessionCenter == nil {
		return nil, errors.New("session center is nil")
	}
	if deps.SubscriptionCenter == nil {
		return nil, errors.New("subscription center is nil")
	}
	if deps.PublishRoute == nil {
		return nil, errors.New("publish route handler is nil")
	}
	return &InProcessClient{
		sessionCenter:      deps.SessionCenter,
		subscriptionCenter: deps.SubscriptionCenter,
		nodeController:     deps.NodeController,
		publishRoute:       deps.PublishRoute,
	}, nil
}

// AcquireSession opens or restores the client's session and makes the calling
// broker the session owner. It runs the connect-time ownership protocol in
// three steps:
//
//  1. OpenSessionForConnect creates or reloads the persistent session state.
//  2. On Clean Start, ReplaceSessionStateOnCleanStart discards the old state.
//  3. TakeOverSessionOwner records this node + owner token as the owner and
//     reports any previous owner so the caller can close that stale connection.
//
// Finally the owner token is mirrored into the subscription center so that
// later subscription writes can be fenced against a stale owner. The request
// carries BrokerNodeID and OwnerToken precisely because ownership is a
// cluster-wide, single-writer concern rather than a local lookup.
func (c *InProcessClient) AcquireSession(
	ctx context.Context,
	req AcquireSessionRequest,
) (*AcquireSessionResponse, error) {
	if err := validateAcquireSessionRequest(req); err != nil {
		return nil, err
	}
	// Step 1: open or reload the persistent session.
	openResp, err := c.sessionCenter.OpenSessionForConnect(ctx, &proto_session.OpenSessionForConnectRequest{
		ClientID:              req.ClientID,
		WillMessage:           req.WillMessage,
		SessionExpiryInterval: req.SessionExpiryInterval,
		NowUnixNano:           req.NowUnixNano,
	})
	if err != nil {
		return nil, err
	}
	// Step 2: Clean Start wipes any prior session state for this client.
	if req.CleanStart {
		if err := c.sessionCenter.ReplaceSessionStateOnCleanStart(ctx, &proto_session.ReplaceSessionStateOnCleanStartRequest{
			ClientID:              req.ClientID,
			WillMessage:           req.WillMessage,
			SessionExpiryInterval: req.SessionExpiryInterval,
			NowUnixNano:           req.NowUnixNano,
		}); err != nil {
			return nil, err
		}
	}
	// Step 3: claim ownership for this node; the response surfaces the
	// previous owner (possibly on another node) when one existed.
	takeoverResp, err := c.sessionCenter.TakeOverSessionOwner(ctx, &proto_session.TakeOverSessionOwnerRequest{
		Owner: &proto_session.SessionOwner{
			ClientID:   req.ClientID,
			NodeID:     req.BrokerNodeID,
			OwnerToken: req.OwnerToken,
			Online:     true,
		},
	})
	if err != nil {
		return nil, err
	}
	// Mirror the owner token into the subscription center so subscription
	// writes share the same fencing token as the session.
	if _, err := c.subscriptionCenter.SetClientOwnerToken(ctx, &proto_topic.SetClientOwnerTokenRequest{
		ClientID:   req.ClientID,
		OwnerToken: req.OwnerToken,
	}); err != nil {
		return nil, err
	}
	return &AcquireSessionResponse{
		OpenSession: openResp,
		Takeover:    takeoverResp,
	}, nil
}

// SaveOfflineState persists the client's state when it disconnects: unfinished
// messages, the outgoing replay cursor, will cleanup and the session expiry.
// The owner token is validated first so a stale owner cannot overwrite the
// state of a session that has already been taken over elsewhere.
func (c *InProcessClient) SaveOfflineState(ctx context.Context, req SaveOfflineStateRequest) error {
	if err := validateClientOwnerRequest(req.ClientID, req.OwnerToken, req.BrokerNodeID); err != nil {
		return err
	}
	return c.sessionCenter.SaveOfflineState(ctx, &proto_session.SaveOfflineStateRequest{
		ClientID:              req.ClientID,
		UnfinishedMessages:    req.UnfinishedMessages,
		OutgoingReplayCursor:  req.OutgoingReplayCursor,
		ClearWill:             req.ClearWill,
		SessionExpiryInterval: req.SessionExpiryInterval,
		NowUnixNano:           req.NowUnixNano,
		OwnerToken:            req.OwnerToken,
	})
}

// Subscribe stores the subscriptions for the owner-held client and returns the
// QoS finally granted per topic. Each subscription is capped at MaxQoS and
// created individually so a single granted-QoS value maps back to each topic.
func (c *InProcessClient) Subscribe(ctx context.Context, req SubscribeRequest) (*SubscribeResponse, error) {
	if err := validateClientOwnerRequest(req.ClientID, req.OwnerToken, req.BrokerNodeID); err != nil {
		return nil, err
	}
	if req.Subscribe == nil {
		return nil, errors.New("subscribe packet is nil")
	}
	subscribePacket := subscribeWithQoSCap(req.Subscribe, req.MaxQoS)
	granted := make([]int32, 0, len(subscribePacket.Subscriptions))
	for _, sub := range subscribePacket.Subscriptions {
		singleReq := &packets.Subscribe{
			PacketID:      subscribePacket.PacketID,
			Properties:    subscribePacket.Properties,
			Subscriptions: []packets.SubOptions{sub},
		}
		rsp, err := c.subscriptionCenter.CreateSub(
			ctx,
			subscription.NewProtoSubRequest(singleReq, req.ClientID, req.OwnerToken),
		)
		if err != nil {
			return nil, err
		}
		ack := int32(-1)
		if rsp != nil && rsp.GetTopics() != nil {
			if value, ok := rsp.GetTopics()[sub.Topic]; ok {
				ack = value
			}
		}
		granted = append(granted, ack)
	}
	return &SubscribeResponse{GrantedQoS: granted}, nil
}

// DeleteClientSubscriptions removes all subscriptions for the owner-held
// client. The owner token guards against a delayed request from an old owner
// deleting the new owner's subscriptions.
func (c *InProcessClient) DeleteClientSubscriptions(
	ctx context.Context,
	req DeleteClientSubscriptionsRequest,
) error {
	if err := validateClientOwnerRequest(req.ClientID, req.OwnerToken, req.BrokerNodeID); err != nil {
		return err
	}
	_, err := c.subscriptionCenter.DeleteClient(ctx, &proto_topic.DeleteClientRequest{
		ClientID:   req.ClientID,
		OwnerToken: req.OwnerToken,
	})
	return err
}

// ListClientSubscriptions returns the subscriptions currently stored for the
// client. It is a read-only query, so it requires only the client id and the
// broker node id and does not check the owner token. A missing result is
// normalized to an empty map rather than nil.
func (c *InProcessClient) ListClientSubscriptions(
	ctx context.Context,
	req ListClientSubscriptionsRequest,
) (*ListClientSubscriptionsResponse, error) {
	if req.ClientID == "" {
		return nil, errors.New("client id is required")
	}
	if req.BrokerNodeID == 0 {
		return nil, errors.New("broker node id is required")
	}
	resp, err := c.subscriptionCenter.GetClientSubscriptions(
		ctx,
		&proto_topic.GetClientSubscriptionsRequest{ClientID: req.ClientID},
	)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.GetTopics() == nil {
		return &ListClientSubscriptionsResponse{Topics: map[string]*proto_topic.SubOption{}}, nil
	}
	return &ListClientSubscriptionsResponse{Topics: resp.GetTopics()}, nil
}

// RoutePublish forwards a message published by the owner-held client to the
// injected PublishRouteHandler after validating owner identity and payload.
func (c *InProcessClient) RoutePublish(ctx context.Context, req RoutePublishRequest) error {
	if err := validateRoutePublishRequest(req); err != nil {
		return err
	}
	return c.publishRoute(ctx, req)
}

// HasMatchingSubscribers reports whether the given topic has any matching
// subscriber (direct plans or shared-group tasks). It is used to decide
// retained-message and will-message delivery without owner-token checks since
// it only reads routing state.
func (c *InProcessClient) HasMatchingSubscribers(
	ctx context.Context,
	req HasMatchingSubscribersRequest,
) (bool, error) {
	if req.ClientID == "" {
		return false, errors.New("client id is required")
	}
	if req.BrokerNodeID == 0 {
		return false, errors.New("broker node id is required")
	}
	if req.Topic == "" {
		return false, errors.New("topic is required")
	}
	router := delivery.NewSubCenterRouter(c.subscriptionCenter)
	resp, err := router.Route(ctx, &packets.Publish{Topic: req.Topic}, req.ClientID)
	if err != nil {
		return false, err
	}
	return resp != nil && (len(resp.Plans) > 0 || len(resp.ShareGroupTasks) > 0), nil
}

// ClosePreviousOwner asks the node that previously owned the session to close
// its stale connection, completing the takeover started by AcquireSession.
// It delegates to the cluster NodeController, addressing the previous owner by
// its node id, client id and owner token, and requires the controller to be
// configured (it is not on single-node deployments).
func (c *InProcessClient) ClosePreviousOwner(ctx context.Context, req ClosePreviousOwnerRequest) error {
	if req.ClientID == "" {
		return errors.New("client id is required")
	}
	if req.BrokerNodeID == 0 {
		return errors.New("broker node id is required")
	}
	if req.PreviousOwner == nil {
		return errors.New("previous owner is required")
	}
	if req.PreviousOwner.GetClientID() == "" {
		return errors.New("previous owner client id is required")
	}
	if req.PreviousOwner.GetOwnerToken() == "" {
		return errors.New("previous owner token is required")
	}
	if c.nodeController == nil {
		return errors.New("node controller is nil")
	}
	return c.nodeController.RequestCloseClient(
		ctx,
		req.PreviousOwner.GetNodeID(),
		req.PreviousOwner.GetClientID(),
		req.PreviousOwner.GetOwnerToken(),
	)
}

// validateAcquireSessionRequest checks owner identity plus the timestamp that
// session state stamping relies on.
func validateAcquireSessionRequest(req AcquireSessionRequest) error {
	if err := validateClientOwnerRequest(req.ClientID, req.OwnerToken, req.BrokerNodeID); err != nil {
		return err
	}
	if req.NowUnixNano <= 0 {
		return errors.New("now unix nano is required")
	}
	return nil
}

// validateRoutePublishRequest checks owner identity and that a publish packet
// is actually present.
func validateRoutePublishRequest(req RoutePublishRequest) error {
	if err := validateClientOwnerRequest(req.ClientID, req.OwnerToken, req.BrokerNodeID); err != nil {
		return err
	}
	if req.Message == nil {
		return errors.New("publish message is nil")
	}
	if req.Message.Publish == nil {
		return errors.New("publish packet is nil")
	}
	return nil
}

// validateClientOwnerRequest enforces the common ownership preconditions for
// write operations: a client id, a fencing owner token, and the calling broker
// node id must all be present.
func validateClientOwnerRequest(clientID string, ownerToken string, brokerNodeID uint64) error {
	if clientID == "" {
		return errors.New("client id is required")
	}
	if ownerToken == "" {
		return errors.New("owner token is required")
	}
	if brokerNodeID == 0 {
		return errors.New("broker node id is required")
	}
	return nil
}

// subscribeWithQoSCap returns a copy of the SUBSCRIBE packet with each
// requested QoS clamped to maxQoS. The original packet is left untouched; when
// maxQoS is out of the valid 0..2 range the packet is returned as-is.
func subscribeWithQoSCap(subscribe *packets.Subscribe, maxQoS int) *packets.Subscribe {
	if subscribe == nil {
		return nil
	}
	if maxQoS < 0 || maxQoS > 2 {
		return subscribe
	}
	cloned := *subscribe
	cloned.Subscriptions = make([]packets.SubOptions, len(subscribe.Subscriptions))
	copy(cloned.Subscriptions, subscribe.Subscriptions)
	for idx := range cloned.Subscriptions {
		if int(cloned.Subscriptions[idx].QoS) > maxQoS {
			cloned.Subscriptions[idx].QoS = byte(maxQoS)
		}
	}
	return &cloned
}

// NewRoutePublishHandler adapts a plain function into a PublishRouteHandler,
// rejecting a nil function so the dependency is validated at wiring time.
func NewRoutePublishHandler(handler func(context.Context, RoutePublishRequest) error) (PublishRouteHandler, error) {
	if handler == nil {
		return nil, fmt.Errorf("publish route handler is nil")
	}
	return PublishRouteHandler(handler), nil
}

// Compile-time assertion that *InProcessClient satisfies the Client interface.
var _ Client = (*InProcessClient)(nil)
