package packetid

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"sync"
)

type PacketIDFactory struct {
	id  uint16
	mux sync.RWMutex
}

func NewPacketIDFactory() *PacketIDFactory {
	var (
		id = randomInitPacketID()
	)
	return &PacketIDFactory{
		id: id,
	}
}

func randomInitPacketID() uint16 {
	// MQTT packet ID范围是1-65535，所以随机生成1-65535的值
	return uint16(rand.Intn(math.MaxUint16-1) + 1)
}

func (p *PacketIDFactory) SetID(id uint16) {
	p.mux.Lock()
	defer p.mux.Unlock()
	p.id = id
}

func (p *PacketIDFactory) NextPacketID() uint16 {
	p.mux.Lock()
	defer p.mux.Unlock()

	if p.id == math.MaxUint16 {
		p.id = 1 // 重置为1，MQTT packet ID范围是1-65535
	} else {
		p.id++
	}

	// 确保不返回0，MQTT协议要求packet ID必须是1-65535
	if p.id == 0 {
		p.id = 1
	}

	return p.id
}

func PacketIDToString(id uint16) string {
	return fmt.Sprintf("%d", id)
}

func StringToPacketID(id string) uint16 {
	i, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return 0
	}
	return uint16(i)
}

type PacketIDTopic struct {
	mux      sync.RWMutex
	packetID map[uint16]string
}

func NewPacketIDTopic() *PacketIDTopic {
	return &PacketIDTopic{
		packetID: make(map[uint16]string),
	}
}

func (p *PacketIDTopic) SetPacketIDTopic(id uint16, topic string) {
	p.mux.Lock()
	defer p.mux.Unlock()
	p.packetID[id] = topic
}

func (p *PacketIDTopic) GetTopic(id uint16) string {
	p.mux.Lock()
	defer p.mux.Unlock()
	return p.packetID[id]
}

func (p *PacketIDTopic) DeletePacketID(id uint16) {
	p.mux.Lock()
	defer p.mux.Unlock()
	delete(p.packetID, id)
}
