package packet

import (
	"github.com/eclipse/paho.golang/packets"
)

type Publish struct {
	MessageID string
	Packet    *packets.Publish
}
