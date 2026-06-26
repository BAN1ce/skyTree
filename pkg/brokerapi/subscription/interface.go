package subscription

import (
	"context"

	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	proto "github.com/BAN1ce/skyTree/proto/proto_topic"
)

type QoSInt32 = int32

type QoSInt = int

type QoS = byte

const (
	QoS0 = QoS(0)
	QoS1 = QoS(1)
	QoS2 = QoS(2)
)

const (
	QoS0Int32 = QoSInt32(0)
	QoS1Int32 = QoSInt32(1)
	QoS2Int32 = QoSInt32(2)
)

const (
	QoS0Int = QoSInt(0)
	QoS1Int = QoSInt(1)
	QoS2Int = QoSInt(2)
)

// SubClient is the interface of the subscription client.
type SubClient interface {
	GetClientID() string
	GetQoS() int32
}

type Center interface {
	CreateSub(ctx context.Context, option *proto.SubRequest) (*proto.SubResponse, error)
	DeleteSub(ctx context.Context, option *proto.UnSubRequest) (*proto.UnSubResponse, error)
	GetAllMatchTopics(ctx context.Context, req *proto.GetAllMatchTopicsRequest) (*proto.GetAllMatchTopicsResponse, error)
	GetAllMatchTopicsForWildTopic(ctx context.Context, req *proto.GetAllMatchTopicsForWildTopicRequest) (*proto.GetAllMatchTopicsForWildTopicResponse, error)
	DeleteClient(ctx context.Context, req *proto.DeleteClientRequest) (*proto.DeleteClientResponse, error)
	DeleteTopic(ctx context.Context, req *proto.DeleteTopicRequest) (*proto.DeleteTopicResponse, error)
	GetAllMatchClient(ctx context.Context, req *proto.GetAllSubTopicClientRequest) (*proto.GetAllSubTopicClientResponse, error)
	// GetAllMatchClientV2 returns per-client multi-match details (does not collapse overlapping subscriptions).
	// Required for the client-centric delivery pipeline.
	GetAllMatchClientV2(ctx context.Context, req *proto.GetAllMatchClientV2Request) (*proto.GetAllMatchClientV2Response, error)
	GetSubTree(ctx context.Context, req *proto.GetSubTreeRequest) (*proto.GetSubTreeResponse, error)
	// SetClientOwnerToken establishes the fencing token for a client, used to fence later operations.
	SetClientOwnerToken(ctx context.Context, req *proto.SetClientOwnerTokenRequest) (*proto.SetClientOwnerTokenResponse, error)
	// GetClientSubscriptions gets all subscriptions for a client
	GetClientSubscriptions(ctx context.Context, req *proto.GetClientSubscriptionsRequest) (*proto.GetClientSubscriptionsResponse, error)
	// GetShareGroupMembers gets all members of a shared subscription group.
	// Members are returned in ascending clientID, then topicFilter order.
	GetShareGroupMembers(ctx context.Context, req *proto.GetShareGroupMembersRequest) (*proto.GetShareGroupMembersResponse, error)
}

func NewProtoSubRequest(subOptions *packets.Subscribe, clientID string, ownerToken string) *proto.SubRequest {
	req := &proto.SubRequest{
		Topics: []*proto.SubOption{},
	}

	// SUBSCRIBE only supports a single SubscriptionIdentifier (MQTT 5.0 spec)
	subID := int32(0)
	if subOptions != nil && subOptions.Properties != nil && subOptions.Properties.SubscriptionIdentifier != nil {
		subID = int32(*subOptions.Properties.SubscriptionIdentifier)
	}

	for i := 0; i < len(subOptions.Subscriptions); i++ {
		req.Topics = append(req.Topics, &proto.SubOption{
			Topic:                  subOptions.Subscriptions[i].Topic,
			QoS:                    int32(subOptions.Subscriptions[i].QoS),
			RetainAsPublished:      subOptions.Subscriptions[i].RetainAsPublished,
			NoLocal:                subOptions.Subscriptions[i].NoLocal,
			RetainHandling:         int32(subOptions.Subscriptions[i].RetainHandling),
			SubscriptionIdentifier: subID,
		})

	}
	req.ClientID = clientID
	req.OwnerToken = ownerToken
	return req
}

func Int32ToQoS(qos int32) QoS {
	switch qos {
	case 1:
		return QoS1
	case 2:
		return QoS2
	case 0:
		return QoS0
	default:
		return QoS0
	}
}
