package event

import (
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/utils"
	"github.com/eclipse/paho.golang/packets"
	"github.com/kataras/go-events"
)

type MQTTEventName = string

const (
	Connect        MQTTEventName = "event.connect"
	Disconnect     MQTTEventName = "event.disconnect"
	Ping           MQTTEventName = "event.ping"
	Pong           MQTTEventName = "event.pong"
	Subscribe      MQTTEventName = "event.subscribe"
	SubscribeAck   MQTTEventName = "event.subscribe_ack"
	Unsubscribe    MQTTEventName = "event.unsubscribe"
	UnsubscribeAck MQTTEventName = "event.unsubscribe_ack"

	ClientPublish         MQTTEventName = "event.client.publish"
	ClientPubAck          MQTTEventName = "event.client.puback"
	ClientPubRel          MQTTEventName = "event.client.pubrel"
	ClientPubRec          MQTTEventName = "event.client.pubrec"
	ClientPubComp         MQTTEventName = "event.client.pubcomp"
	ClientPublishTopic    MQTTEventName = "event.client.publish_topic"
	BrokerPublish         MQTTEventName = "event.store.publish"
	BrokerPublishToClient MQTTEventName = "event.store.publish_to_client"
	ClientPublishAck      MQTTEventName = "event.client.publish_ack"
	BrokerPublishAck      MQTTEventName = "event.store.publish_ack"
	ClientAuth            MQTTEventName = "event.client.auth"
	BrokerAuth            MQTTEventName = "event.store.auth"
)

var (
	packetTypeNumberMap = map[byte]MQTTEventName{
		packets.CONNECT:    Connect,
		packets.DISCONNECT: Disconnect,
		packets.PINGREQ:    Ping,
		packets.PINGRESP:   Pong,
		packets.SUBSCRIBE:  Subscribe,
		packets.SUBACK:     SubscribeAck,

		packets.UNSUBSCRIBE: Unsubscribe,
		packets.UNSUBACK:    UnsubscribeAck,

		packets.PUBLISH: ClientPublish,
		packets.PUBACK:  ClientPubAck,
		packets.PUBREL:  ClientPubRel,
		packets.PUBREC:  ClientPubRec,
		packets.PUBCOMP: ClientPubComp,
		packets.AUTH:    ClientAuth,
	}
)

func (e *Event) EmitClientMQTTEvent(packetType byte, packet packets.Packet) {
	if event, ok := packetTypeNumberMap[packetType]; ok {
		Driver.Emit(events.EventName(event), packet)
		return
	}
	logger.Logger.Error("unknown packet type")
}

func ReceivedTopicPublishEventName(topic string) events.EventName {
	return utils.WithEventPrefix(ClientPublishTopic, topic)
}
