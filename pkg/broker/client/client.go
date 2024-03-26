package client

import (
	"github.com/BAN1ce/skyTree/pkg/broker/session"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/eclipse/paho.golang/packets"
)

type ID interface {
	GetID() string
}

type PacketWriter interface {
	// WritePacket writes the packet to the writer.
	// Warning: packetID is original packetID, method should change it to the new one that does not used.
	WritePacket(packet packets.Packet) (err error)
	ID
	Close() error
}

type Client interface {
	Publish(publish *packet.Message) error
	PubRel(message *packet.Message) error

	GetUnFinishedMessage() []*packet.Message
	GetSession() session.Session
	GetPacketWriter() PacketWriter
	Handle
}

type Handle interface {
	HandlePublishAck(pubAck *packets.Puback)
	HandlePublishRec(pubRec *packets.Pubrec)
	HandelPublishComp(pubComp *packets.Pubcomp)
}
