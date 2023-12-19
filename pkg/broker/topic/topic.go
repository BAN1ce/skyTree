package topic

import (
	"context"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/BAN1ce/skyTree/pkg/utils"
	"github.com/eclipse/paho.golang/packets"
)

type Topic interface {
	Start(ctx context.Context) error
	Close() error
	Publish(publish *packet.Message) error
	GetUnFinishedMessage() []*packet.Message
}

type TopicQoS1 interface {
	Topic
	HandlePublishAck(puback *packets.Puback)
}

type TopicQoS2 interface {
	Topic
	HandlePublishRec(pubrec *packets.Pubrec)
	HandlePublishComp(pubcomp *packets.Pubcomp)
}

func SplitShareAndNoShare(subPacket *packets.Subscribe) (shareSubscribe *packets.Subscribe, noShareSubscribe *packets.Subscribe) {
	var (
		shareTopic   = make([]packets.SubOptions, 0)
		noShareTopic = make([]packets.SubOptions, 0)
	)
	shareSubscribe = &packets.Subscribe{
		PacketID:   subPacket.PacketID,
		Properties: subPacket.Properties,
	}
	noShareSubscribe = &packets.Subscribe{
		PacketID:   subPacket.PacketID,
		Properties: subPacket.Properties,
	}
	for _, topic := range subPacket.Subscriptions {
		if utils.IsShareTopic(topic.Topic) {
			shareTopic = append(shareTopic, topic)
		} else {
			noShareTopic = append(noShareTopic, topic)
		}
	}
	shareSubscribe.Subscriptions = shareTopic
	noShareSubscribe.Subscriptions = noShareTopic
	return shareSubscribe, noShareSubscribe
}
