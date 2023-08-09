package packet

import (
	"github.com/eclipse/paho.golang/packets"
)

type PublishMessage struct {
	MessageID string
	Packet    *packets.Publish
	TimeStamp int64
}
