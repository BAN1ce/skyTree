package broker

// PublishListener is the interface of the publish event listener.
// It is used to listen the publish event from broker.
// The publish event will be triggered when the client publish a message to the broker.
type PublishListener interface {
	CreatePublishEvent(topic string, handler func(i ...interface{}))
	DeletePublishEvent(topic string, handler func(i ...interface{}))
}
