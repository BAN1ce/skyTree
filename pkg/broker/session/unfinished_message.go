package session

import (
	"github.com/BAN1ce/skyTree/pkg/packet"
)

type UnFinishedMessage struct {
	Message     *packet.Message
	MessageID   string
	PacketID    string
	PubReceived bool
}
