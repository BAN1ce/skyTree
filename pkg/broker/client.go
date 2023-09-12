package broker

import "github.com/eclipse/paho.golang/packets"

type PublishWriter interface {
	// WritePacket writes the packet to the writer.
	// Warning: packetID is original packetID, method should change it to the new one that does not used.
	WritePacket(packet packets.Packet)

	GetID() string
	Close() error
}
