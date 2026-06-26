package packetpool

import (
	"sync"

	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

var (
	PubRelPool = NewPubRel()
)

type PubRel struct {
	sync.Pool
}

func (p *PubRel) Get() *packets.ControlPacket {
	return p.Pool.Get().(*packets.ControlPacket)
}

func (p *PubRel) Put(b *packets.ControlPacket) {
	if pubrel, ok := b.Content.(*packets.Pubrel); ok {
		pubrel.PacketID = 0
	}
	p.Pool.Put(b)
}

func NewPubRel() *PubRel {
	return &PubRel{
		sync.Pool{
			New: func() interface{} {
				return packets.NewControlPacket(packets.PUBREL)
			},
		},
	}
}
