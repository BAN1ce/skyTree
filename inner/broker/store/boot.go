package store

import (
	"github.com/BAN1ce/skyTree/pkg/broker"
)

var (
	DefaultMessageStore      broker.TopicMessageStore
	DefaultMessageStoreEvent broker.MessageStoreEvent
	DefaultSerializerVersion broker.SerialVersion
)

func Boot(store broker.TopicMessageStore, event broker.MessageStoreEvent) {
	DefaultMessageStore = store
	DefaultMessageStoreEvent = event
	DefaultSerializerVersion = broker.SerialVersion1
}
