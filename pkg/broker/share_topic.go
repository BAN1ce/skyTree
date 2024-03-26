package broker

import (
	"github.com/BAN1ce/skyTree/pkg/broker/client"
	"github.com/BAN1ce/skyTree/pkg/broker/topic"
	"github.com/eclipse/paho.golang/packets"
)

type ShareClient interface {
	client.PacketWriter
}

type ShareTopicManager interface {
	CreateShareTopic(topic string) (ShareTopic, error)
	DeleteShareTopic(topic string)
}

type ShareTopic interface {
	ShareTopicAddClient(client ShareClient, qos QoS)
	ShareTopicRemoveClient(id string)
	topic.Topic
}

type ShareTopicQoS1 interface {
	ShareTopic
	HandlePublishAck(puback *packets.Puback)
}

type ShareTopicQoS2 interface {
	ShareTopic
	HandlePublishRec(pubrec *packets.Pubrec)
	HandlePublishComp(pubcomp *packets.Pubcomp)
}

type ShareTopics interface {
	Sub(options packets.SubOptions, client ShareClient) error
	UnSub(topic string, clientID string) error
	Close() error
}
