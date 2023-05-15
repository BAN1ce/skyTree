package client

import "github.com/eclipse/paho.golang/packets"

type waitAckPacket struct {
	packet    *packets.Publish
	messageID string
	ack       bool
}

func newWaitAckPacket(packet *packets.Publish, id string) *waitAckPacket {
	return &waitAckPacket{
		packet:    packet,
		messageID: id,
	}
}
