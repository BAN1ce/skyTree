package packet

import (
	"github.com/eclipse/paho.golang/packets"
)

type PublishMessage struct {
	MessageID     string
	PublishPacket *packets.Publish
	PubRelPacket  *packets.Pubrel
	TimeStamp     int64
	PubReceived   bool
	FromSession   bool
}
