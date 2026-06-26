package notify

import (
	"context"
	"errors"

	deliveryevent "github.com/BAN1ce/skyTree/internal/broker/delivery/event"
	"github.com/BAN1ce/skyTree/pkg/cluster"
	"github.com/BAN1ce/skyTree/pkg/eventbus"
)

var (
	ErrLocalEventCenterNil = errors.New("delivery event: local event center is nil")
	ErrClientIDEmpty       = errors.New("delivery event: clientID is empty")
	ErrHandlerNil          = errors.New("delivery event: handler is nil")
	ErrListenerIDEmpty     = errors.New("delivery event: listener id is empty")
	ErrClientIDsEmpty      = errors.New("delivery event: clientIDs is empty")
)

// ClientDeliveryOptions contains MQTT5 subscription options for a specific client.
type ClientDeliveryOptions struct {
	NoLocal             bool
	RAP                 bool
	SubscriptionIDsJSON string
}

// NotifyHandler handles one client delivery event payload.
type NotifyHandler func(*deliveryevent.Notify)

// ClientDeliveryEvent is the bridge between:
// - local EventCenter (emit/listen per clientID)
// - cluster NodeController (gRPC) for cross-node notify
type ClientDeliveryEvent interface {
	AddListener(ctx context.Context, clientID string, handler NotifyHandler) (id string, meta string, err error)
	DeleteListener(ctx context.Context, clientID string, id string) error

	NotifyToNode(ctx context.Context, nodeID uint64, publishTopic string, clientIDs []string, kind deliveryevent.Kind, payload []byte, clientOptions map[string]ClientDeliveryOptions) error
	NotifySharedWake(ctx context.Context, payload SharedWakePayload) error
}

type clientDeliveryEvent struct {
	localNodeID uint64
	localEvent  *eventbus.EventCenter[*deliveryevent.Notify]
	notify      cluster.NodeController
}

func New(localNodeID uint64, localEvent *eventbus.EventCenter[*deliveryevent.Notify], notify cluster.NodeController) ClientDeliveryEvent {
	return &clientDeliveryEvent{
		localNodeID: localNodeID,
		localEvent:  localEvent,
		notify:      notify,
	}
}

func (c *clientDeliveryEvent) AddListener(ctx context.Context, clientID string, handler NotifyHandler) (id string, meta string, err error) {
	_ = ctx
	if c.localEvent == nil {
		return "", "", ErrLocalEventCenterNil
	}
	if clientID == "" {
		return "", "", ErrClientIDEmpty
	}
	if handler == nil {
		return "", "", ErrHandlerNil
	}
	eventName := eventbus.ReceiveClientDeliveryEventName(clientID)
	id, meta = c.localEvent.AddListener(eventName, func(notifyPayload *deliveryevent.Notify) {
		if notifyPayload == nil {
			return
		}
		handler(notifyPayload)
	})
	return id, meta, nil
}

func (c *clientDeliveryEvent) DeleteListener(ctx context.Context, clientID string, id string) error {
	_ = ctx
	if c.localEvent == nil {
		return ErrLocalEventCenterNil
	}
	if clientID == "" {
		return ErrClientIDEmpty
	}
	if id == "" {
		return ErrListenerIDEmpty
	}
	eventName := eventbus.ReceiveClientDeliveryEventName(clientID)
	c.localEvent.DeleteListener(eventName, id)
	return nil
}

func (c *clientDeliveryEvent) NotifyToNode(
	ctx context.Context,
	nodeID uint64,
	publishTopic string,
	clientIDs []string,
	kind deliveryevent.Kind,
	payload []byte,
	clientOptions map[string]ClientDeliveryOptions,
) error {
	if len(clientIDs) == 0 {
		return ErrClientIDsEmpty
	}

	// Local fast-path: emit into local EventCenter.
	if nodeID == 0 || nodeID == c.localNodeID || c.notify == nil {
		if c.localEvent == nil {
			return ErrLocalEventCenterNil
		}
		baseNotify, emitKey, err := BuildPreparedNotify(kind, publishTopic, payload)
		if err != nil {
			return err
		}

		for _, clientID := range clientIDs {
			if clientID == "" {
				continue
			}
			eventName := eventbus.ReceiveClientDeliveryEventName(clientID)
			notifyPayload := BuildClientNotifyPayload(baseNotify, clientID, clientOptions)
			if err := c.localEvent.Emit(eventName, emitKey, notifyPayload); err != nil {
				return err
			}
		}
		return nil
	}

	// Remote: delegate to NodeController (gRPC).
	// Convert clientOptions to cluster.ClientDeliveryOptions.
	clusterOptions := make(map[string]cluster.ClientDeliveryOptions, len(clientOptions))
	for clientID, opts := range clientOptions {
		clusterOptions[clientID] = cluster.ClientDeliveryOptions{
			NoLocal:             opts.NoLocal,
			RAP:                 opts.RAP,
			SubscriptionIDsJSON: opts.SubscriptionIDsJSON,
		}
	}
	return c.notify.NotifyClientDelivery(ctx, nodeID, publishTopic, clientIDs, int32(kind), payload, clusterOptions)
}

func (c *clientDeliveryEvent) NotifySharedWake(ctx context.Context, payload SharedWakePayload) error {
	if _, err := EncodeSharedWakePayload(payload); err != nil {
		return err
	}
	if c == nil || c.notify == nil {
		return nil
	}
	return c.notify.NotifySharedSubscriptionWake(ctx, cluster.SharedSubscriptionWake{
		ShareGroup:  payload.ShareGroup,
		TopicFilter: payload.TopicFilter,
		TaskID:      payload.TaskID,
	})
}
