package util

import (
	"math"
	"math/rand"
	"sync/atomic"
)

func GeneratePacketID() uint16 {
	return uint16(rand.Intn(65535))
}

type PacketIDFactory struct {
	id *atomic.Uint32
}

func NewPacketIDFactory() *PacketIDFactory {
	var (
		id atomic.Uint32
	)
	// TODO: first packet id should be randomInitPacketID
	id.Store(randomInitPacketID())
	return &PacketIDFactory{
		id: &id,
	}
}

func randomInitPacketID() uint32 {
	return uint32(rand.Intn(math.MaxUint32))
}

func (p *PacketIDFactory) SetID(id uint16) {
	p.id.Store(uint32(id))
}

func (p *PacketIDFactory) ReadID() uint16 {
	return uint16(p.id.Load())
}

func (p *PacketIDFactory) Generate() uint16 {
	var (
		newID uint16
	)
	id := p.id.Add(1)
	if id == 0x0000 {
		return p.Generate()
	}
	if id > 0x00FF {
		newID = uint16(id >> 16)
	} else {
		newID = uint16(id)
	}
	p.id.Store(uint32(newID))
	return newID
}
