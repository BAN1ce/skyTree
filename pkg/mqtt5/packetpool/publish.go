package packetpool

import (
	"sync"

	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

var (
	PublishPool = NewPublish()
)

type Publish struct {
	sync.Pool
}

func (p *Publish) Get() *packets.ControlPacket {
	return p.Pool.Get().(*packets.ControlPacket)
}

func (p *Publish) Put(b *packets.ControlPacket) {
	if publish, ok := b.Content.(*packets.Publish); ok {
		publish.Duplicate = false
		publish.QoS = 0
		publish.Retain = false
		publish.PacketID = 0
		publish.Payload = nil
		publish.Topic = ""
		publish.Properties = nil
	}
	p.Pool.Put(b)
}

func NewPublish() *Publish {
	return &Publish{
		sync.Pool{
			New: func() interface{} {
				return packets.NewControlPacket(packets.PUBLISH)
			},
		},
	}
}

func CopyPublish(dest *packets.ControlPacket, src *packets.ControlPacket) {
	destPublish, ok1 := dest.Content.(*packets.Publish)
	srcPublish, ok2 := src.Content.(*packets.Publish)
	if ok1 && ok2 {
		destPublish.Duplicate = srcPublish.Duplicate
		destPublish.QoS = srcPublish.QoS
		destPublish.Retain = srcPublish.Retain
		destPublish.Topic = srcPublish.Topic
		destPublish.PacketID = srcPublish.PacketID
		destPublish.Payload = srcPublish.Payload
		copyProperties(destPublish, srcPublish)
	}
}

// copyProperties copies PublishProperties from src to dest, including deep copy
func copyProperties(dest *packets.Publish, src *packets.Publish) {
	if src.Properties == nil {
		dest.Properties = nil
		return
	}

	// Create new PublishProperties struct
	dest.Properties = &packets.PublishProperties{}

	// Copy all pointer fields
	if src.Properties.PayloadFormat != nil {
		val := *src.Properties.PayloadFormat
		dest.Properties.PayloadFormat = &val
	}
	if src.Properties.MessageExpiry != nil {
		val := *src.Properties.MessageExpiry
		dest.Properties.MessageExpiry = &val
	}
	if src.Properties.TopicAlias != nil {
		val := *src.Properties.TopicAlias
		dest.Properties.TopicAlias = &val
	}

	// Copy string fields
	dest.Properties.ContentType = src.Properties.ContentType
	dest.Properties.ResponseTopic = src.Properties.ResponseTopic

	// Deep copy byte slices
	if src.Properties.CorrelationData != nil {
		dest.Properties.CorrelationData = make([]byte, len(src.Properties.CorrelationData))
		copy(dest.Properties.CorrelationData, src.Properties.CorrelationData)
	}

	if len(src.Properties.SubscriptionIdentifier) > 0 {
		dest.Properties.SubscriptionIdentifier = make([]int, len(src.Properties.SubscriptionIdentifier))
		copy(dest.Properties.SubscriptionIdentifier, src.Properties.SubscriptionIdentifier)
	}

	// Deep copy User slice to avoid data race
	if len(src.Properties.User) > 0 {
		dest.Properties.User = make([]packets.User, len(src.Properties.User))
		copy(dest.Properties.User, src.Properties.User)
	}
}
