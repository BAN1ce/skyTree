package client

import "github.com/eclipse/paho.golang/packets"

type QoS2Message struct {
	publishPacket *packets.Publish
}

type QoS2Handler struct {
	waiting map[uint16]*QoS2Message
}

func NewQoS2Handler() *QoS2Handler {
	return &QoS2Handler{
		waiting: make(map[uint16]*QoS2Message),
	}
}

func (w *QoS2Handler) HandlePublish(publish *packets.Publish) bool {
	if _, ok := w.waiting[publish.PacketID]; ok {
		return false
	}
	w.waiting[publish.PacketID] = &QoS2Message{
		publishPacket: publish,
	}
	return true
}
func (w *QoS2Handler) HandlePubRel(pubRel *packets.Pubrel) (*packets.Publish, bool) {
	if message, ok := w.waiting[pubRel.PacketID]; !ok {
		return message.publishPacket, false
	} else {
		delete(w.waiting, pubRel.PacketID)
		return message.publishPacket, true
	}
}
