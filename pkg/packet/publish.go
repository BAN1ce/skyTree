package packet

import (
	"github.com/eclipse/paho.golang/packets"
)

type PublishMessage struct {
	ClientID      string
	MessageID     string
	PublishPacket *packets.Publish
	PubRelPacket  *packets.Pubrel
	PubReceived   bool
	FromSession   bool
	TimeStamp     int64
	ExpiredTime   int64
	Will          bool
}
