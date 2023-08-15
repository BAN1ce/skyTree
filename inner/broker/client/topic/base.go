package topic

type meta struct {
	topic           string
	qos             byte
	windowSize      int
	writer          PublishWriter
	latestMessageID string
}

type PrepareMetaData interface {
	ReadTopicLatestPushedMessageID(topic string) (string, bool)
	ReadTopicUnAckMessageID(topic string) []string
	ReadTopicUnRecPacketID(topic string) []string
	ReadTopicUnCompPacketID(topic string) []string
}

type QoS1Session interface {
	CreateTopicUnAckMessageID(topic string, messageID []string)
	SetTopicLatestPushedMessageID(topic string, messageID string)
	ReadTopicLatestPushedMessageID(topic string) (string, bool)
	ReadTopicUnAckMessageID(topic string) []string
}
