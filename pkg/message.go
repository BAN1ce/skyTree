package pkg

import (
	"github.com/eclipse/paho.golang/packets"
)

type Packet struct {
	id        string
	topic     string
	pubPacket *packets.Publish
}

func NewPacket(id, topic string, pubPacket *packets.Publish) *Packet {
	return &Packet{
		id:        id,
		topic:     topic,
		pubPacket: pubPacket,
	}
}

func (p *Packet) GetID() string {
	return p.id
}

func (p *Packet) GetPayload() []byte {
	return p.pubPacket.Payload
}

func (p *Packet) GetTopic() string {
	return p.topic
}
func (p *Packet) PubPacket() *packets.Publish {
	return p.pubPacket
}
func (p *Packet) GetQoS() byte {
	return p.pubPacket.QoS
}

func (p *Packet) Decode() packets.Packet {
	return p.pubPacket
}

type Message interface {
	GetID() string
	GetPayload() []byte
	GetTopic() string
	GetQoS() byte
	Decode() packets.Packet
}
