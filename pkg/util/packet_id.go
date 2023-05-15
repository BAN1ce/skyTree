package util

import (
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
	id.Store(uint32(GeneratePacketID()))
	return &PacketIDFactory{
		id: &id,
	}
}

func (p *PacketIDFactory) Generate() uint16 {
	id := p.id.Add(1)
	if id == 0x0000 {
		return p.Generate()
	}
	if id > 0xFF {
		return uint16(id >> 16)
	} else {
		return uint16(id)
	}
}
