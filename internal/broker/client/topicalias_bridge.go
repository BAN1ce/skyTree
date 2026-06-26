package client

import topicalias "github.com/BAN1ce/skyTree/internal/broker/client/internal/topicalias"

type TopicAliasManager = topicalias.TopicAliasManager

func NewTopicAliasManager() *TopicAliasManager {
	return topicalias.NewTopicAliasManager()
}
