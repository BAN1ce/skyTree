package broker

import (
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/eclipse/paho.golang/packets"
)

type ClientID interface {
	GetID() string
}

type PacketWriter interface {
	// WritePacket writes the packet to the writer.
	// Warning: packetID is original packetID, method should change it to the new one that does not used.
	WritePacket(packet packets.Packet) (err error)
	ClientID
	Close() error
}

type Client interface {
	Publish(publish *packet.Message) error
	PubRel(message *packet.Message) error
	GetPacketWriter() PacketWriter
	HandlePublishAck(pubAck *packets.Puback)
	HandlePublishRec(pubRec *packets.Pubrec)
	HandelPublishComp(pubComp *packets.Pubcomp)
	GetUnFinishedMessage() []*packet.Message
}
