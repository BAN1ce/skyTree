package core

import (
	"github.com/BAN1ce/skyTree/inner/broker/client/topic"
	"github.com/BAN1ce/skyTree/inner/broker/message_source"
	"github.com/BAN1ce/skyTree/inner/broker/store"
	"github.com/BAN1ce/skyTree/inner/event"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/broker"
	topic2 "github.com/BAN1ce/skyTree/pkg/broker/topic"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
)

func CreateTopic(client broker.PacketWriter, options *packets.SubOptions) topic2.Topic {
	// TODO: unfinished message
	switch options.QoS {
	case broker.QoS0:
		return topic.NewQoS0(options, client, message_source.NewEventSource(options.Topic, event.GlobalEvent))
	case broker.QoS1:
		return topic.NewQoS1(options, client, message_source.NewStoreSource(options.Topic, store.DefaultMessageStore, store.DefaultMessageStoreEvent), nil)
	case broker.QoS2:
		return topic.NewQoS2(options, client, message_source.NewStoreSource(options.Topic, store.DefaultMessageStore, store.DefaultMessageStoreEvent), nil)
	default:
		logger.Logger.Error("create topic failed, qos not support", zap.String("topic", options.Topic), zap.Uint8("qos", options.QoS))
		return nil
	}
}

func CreateTopicFromSession(client broker.PacketWriter, options *packets.SubOptions, unfinished []*packet.Message, latestMessageID string) topic2.Topic {
	switch options.QoS {
	case broker.QoS0:
		return topic.NewQoS0(options, client, message_source.NewEventSource(options.Topic, event.GlobalEvent))
	case broker.QoS1:
		return topic.NewQoS1(options, client, message_source.NewStoreSource(options.Topic, store.DefaultMessageStore, store.DefaultMessageStoreEvent), unfinished, topic.QoS1WithLatestMessageID(latestMessageID))
	case broker.QoS2:
		return topic.NewQoS2(options, client, message_source.NewStoreSource(options.Topic, store.DefaultMessageStore, store.DefaultMessageStoreEvent), unfinished, topic.QoS2WithLatestMessageID(latestMessageID))
	default:
		logger.Logger.Error("create topic failed, qos not support", zap.String("topic", options.Topic), zap.Uint8("qos", options.QoS))
		return nil
	}

}
