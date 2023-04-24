package pool

import (
	"github.com/eclipse/paho.golang/packets"
	"sync"
)

type PublishPool struct {
	sync.Pool
}

func (p *PublishPool) Get() *packets.Publish {
	return p.Pool.Get().(*packets.Publish)
}

func (p *PublishPool) Put(b *packets.Publish) {
	p.Pool.Put(b)
}

func NewPublishPool() *PublishPool {
	return &PublishPool{
		sync.Pool{
			New: func() interface{} {
				return packets.NewControlPacket(packets.PUBLISH).Content.(*packets.Publish)
			},
		},
	}
}
