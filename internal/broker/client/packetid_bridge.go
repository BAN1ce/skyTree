package client

import packetid "github.com/BAN1ce/skyTree/internal/broker/client/internal/packetid"

type PacketIDFactory = packetid.PacketIDFactory
type PacketIDTopic = packetid.PacketIDTopic

func NewPacketIDFactory() *PacketIDFactory {
	return packetid.NewPacketIDFactory()
}

func PacketIDToString(id uint16) string {
	return packetid.PacketIDToString(id)
}

func StringToPacketID(id string) uint16 {
	return packetid.StringToPacketID(id)
}

func NewPacketIDTopic() *PacketIDTopic {
	return packetid.NewPacketIDTopic()
}
