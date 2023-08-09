package topic

import (
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/BAN1ce/skyTree/pkg/pool"
	"github.com/eclipse/paho.golang/packets"
)

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

func copyPublish(dest, publish *packets.Publish) *packets.Publish {
	pool.CopyPublish(dest, publish)
	dest.QoS = pkg.QoS0
	return dest
}
