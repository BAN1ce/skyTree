package qos

import "github.com/eclipse/paho.golang/packets"

type QoS1Queue interface {
	Push(messageID string, packet *packets.Publish) error
	Ack(ack *packets.Puback) (messageID string, ok bool)
	UnAckMessageID() []string
	Close() error
}
