package packet

import (
	"github.com/BAN1ce/skyTree/pkg/errs"
	"github.com/BAN1ce/skyTree/pkg/pool"
	"github.com/eclipse/paho.golang/packets"
)

type Publish struct {
	pub *packets.Publish
}

func NewPublish(publish *packets.Publish) *Publish {
	return &Publish{
		pub: publish,
	}
}

func (p *Publish) GetDup() bool {
	return p.pub.Duplicate
}

func (p *Publish) GetQoS() byte {
	return p.pub.QoS
}

func (p *Publish) GetRetain() bool {
	return p.pub.Retain
}

func (p *Publish) GetTopic() string {
	return p.pub.Topic
}

func (p *Publish) GetIdentifier() uint16 {
	return p.pub.PacketID
}

func (p *Publish) GetPayload() []byte {
	return p.pub.Payload
}

func (p *Publish) GetProperties() *packets.Properties {
	return p.pub.Properties
}

func (p *Publish) Encode() ([]byte, error) {
	var (
		bf = pool.ByteBufferPool.Get()
	)
	defer pool.ByteBufferPool.Put(bf)
	if _, err := p.pub.WriteTo(bf); err != nil {
		return nil, err
	} else {
		return bf.Bytes(), nil
	}
}
func (p *Publish) Decode(data []byte) error {
	var (
		bf = pool.ByteBufferPool.Get()
	)
	defer pool.ByteBufferPool.Put(bf)
	bf.Write(data)

	if pub, err := packets.ReadPacket(bf); err != nil {
		return err
	} else {
		if pub.FixedHeader.Type == packets.PUBLISH {
			p.pub, _ = pub.Content.(*packets.Publish)
			return nil
		} else {
			return errs.ErrorInvalidPacket
		}
	}
}
