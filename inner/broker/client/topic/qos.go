package topic

type meta struct {
	topic      string
	qos        byte
	windowSize int
	writer     PublishWriter
}
