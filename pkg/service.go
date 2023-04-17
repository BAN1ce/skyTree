package pkg

import "github.com/BAN1ce/skyTree/pkg/proto"

type SubscriptionTree interface {
	AddClientSubscription(clientID string, topics map[string]int32, meta map[string]string) error

	RemoveClientSubscription(clientID string) error

	GetSubscriptionTree(topic string) (*proto.TreeNode, error)

	GetHashSubTopic(topicSection string) (map[string]int64, error)

	GetTopicSubscribers(topic string) ([]*proto.Client, error)
}
