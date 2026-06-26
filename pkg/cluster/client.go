package cluster

import (
	"context"

	"github.com/lni/dragonboat/v3/statemachine"
)

type Client interface {
	Write(ctx context.Context, data []byte) (statemachine.Result, error)
	Read(ctx context.Context, query interface{}) (interface{}, error)
	GetNodeID() uint64
}

type ClientDeliveryOptions struct {
	NoLocal             bool
	RAP                 bool
	SubscriptionIDsJSON string
}

type SharedSubscriptionWake struct {
	ShareGroup  string `json:"shareGroup"`
	TopicFilter string `json:"topicFilter"`
	TaskID      string `json:"taskID"`
}

type NodeController interface {
	// NotifyClientDelivery notifies the target node to wake (or directly deliver QoS0) for specific clientIDs.
	// kind matches proto.ClientDeliveryNotifyKind numeric values, but uses int32 here to avoid importing grpc/proto.
	// clientOptions maps clientID to MQTT5 subscription options (NoLocal, RAP). Can be nil.
	NotifyClientDelivery(ctx context.Context, nodeID uint64, publishTopic string, clientIDs []string, kind int32, payload []byte, clientOptions map[string]ClientDeliveryOptions) error
	// NotifySharedSubscriptionWake broadcasts a shared subscription task wake to peer broker nodes.
	NotifySharedSubscriptionWake(ctx context.Context, wake SharedSubscriptionWake) error
	// RequestCloseClient requests a remote node to close a specific client connection.
	// ownerToken is a fencing token (UUID string). When non-empty, the remote node must only close
	// the connection if its current token matches (prevents delayed/duplicate closes from killing a newer owner).
	RequestCloseClient(ctx context.Context, nodeID uint64, clientID string, ownerToken string) error
}
