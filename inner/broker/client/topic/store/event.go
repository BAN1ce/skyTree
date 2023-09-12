package store

type Event interface {
	CreateListenMessageStoreEvent(topic string, handler func(...interface{}))
	DeleteListenMessageStoreEvent(topic string, handler func(i ...interface{}))
}
