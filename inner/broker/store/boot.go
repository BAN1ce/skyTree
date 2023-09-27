package store

import (
	"github.com/BAN1ce/skyTree/pkg/broker"
	"github.com/BAN1ce/skyTree/pkg/broker/message/serializer"
)

var (
	DefaultMessageStore      broker.TopicMessageStore
	DefaultMessageStoreEvent broker.MessageStoreEvent
	DefaultSerializerVersion serializer.SerialVersion
)

func Boot(store broker.TopicMessageStore, event broker.MessageStoreEvent) {
	DefaultMessageStore = store
	DefaultMessageStoreEvent = event
	DefaultSerializerVersion = serializer.ProtoBufVersion
}
