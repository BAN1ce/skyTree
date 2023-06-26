package pool

import (
	"github.com/eclipse/paho.golang/packets"
	"sync"
)

var (
	PublishPool = NewPublish()
)

type Publish struct {
	sync.Pool
}

func (p *Publish) Get() *packets.Publish {
	return p.Pool.Get().(*packets.Publish)
}

func (p *Publish) Put(b *packets.Publish) {
	p.Pool.Put(b)
}

func NewPublish() *Publish {
	return &Publish{
		sync.Pool{
			New: func() interface{} {
				return packets.NewControlPacket(packets.PUBLISH).Content.(*packets.Publish)
			},
		},
	}
}

func CopyPublish(publish *packets.Publish) *packets.Publish {
	var (
		p = PublishPool.Get()
	)
	p.Duplicate = publish.Duplicate
	p.QoS = publish.QoS
	p.Retain = publish.Retain
	p.Topic = publish.Topic
	p.PacketID = publish.PacketID
	p.Payload = publish.Payload
	p.Properties = publish.Properties
	return p
}
