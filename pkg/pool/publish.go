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

func CopyPublish(dest *packets.Publish, src *packets.Publish) {
	dest.Duplicate = src.Duplicate
	dest.QoS = src.QoS
	dest.Retain = src.Retain
	dest.Topic = src.Topic
	dest.PacketID = src.PacketID
	dest.Payload = src.Payload
	dest.Properties = src.Properties
}
