package client

import "github.com/eclipse/paho.golang/packets"

type QoS2Message struct {
	publishPacket *packets.Publish
}

type HandleQoS2 struct {
	waiting map[uint16]*QoS2Message
}

func NewHandleQoS2() *HandleQoS2 {
	return &HandleQoS2{
		waiting: make(map[uint16]*QoS2Message),
	}
}

func (w *HandleQoS2) HandlePublish(publish *packets.Publish) bool {
	if _, ok := w.waiting[publish.PacketID]; ok {
		return false
	}
	w.waiting[publish.PacketID] = &QoS2Message{
		publishPacket: publish,
	}
	return true
}
func (w *HandleQoS2) HandlePubRel(pubrel *packets.Pubrel) (*packets.Publish, bool) {
	if message, ok := w.waiting[pubrel.PacketID]; !ok {
		return message.publishPacket, false
	} else {
		delete(w.waiting, pubrel.PacketID)
		return message.publishPacket, true
	}
}
