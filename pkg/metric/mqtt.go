package metric

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	MQTTPacketsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_mqtt_packets_total",
		Help: "Total number of MQTT packets by direction and packet type.",
	}, []string{"direction", "packet_type"})

	MQTTPublishPacketsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_mqtt_publish_packets_total",
		Help: "Total number of MQTT PUBLISH packets by QoS and direction.",
	}, []string{"qos", "direction"})

	MQTTDroppedMessagesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "skytree_mqtt_dropped_messages_total",
		Help: "Total number of dropped MQTT messages.",
	}, []string{"qos", "reason"})

	packetTypeString = [...]string{
		"",
		"CONNECT",
		"CONNACK",
		"PUBLISH",
		"PUBACK",
		"PUBREC",
		"PUBREL",
		"PUBCOMP",
		"SUBSCRIBE",
		"SUBACK",
		"UNSUBSCRIBE",
		"UNSUBACK",
		"PINGREQ",
		"PINGRESP",
		"DISCONNECT",
		"AUTH",
	}
)

func RecordMQTTPublishPacket(qos byte, direction string) {
	MQTTPublishPacketsTotal.WithLabelValues(normalizeMQTTQoS(qos), normalizeMQTTDirection(direction)).Inc()
}

func RecordMQTTReceivedPacket(packetType byte) {
	RecordMQTTPacket("in", packetType)
}

func RecordMQTTSentPacket(packetType byte) {
	RecordMQTTPacket("out", packetType)
}

func RecordMQTTPacket(direction string, packetType byte) {
	MQTTPacketsTotal.WithLabelValues(normalizeMQTTDirection(direction), packetTypeLabel(packetType)).Inc()
}

func RecordMQTTDroppedMessage(qos byte, reason string) {
	MQTTDroppedMessagesTotal.WithLabelValues(normalizeMQTTQoS(qos), normalizeMQTTDropReason(reason)).Inc()
}

func packetTypeLabel(packetType byte) string {
	if packetType == 0 || packetType > 15 {
		return unknownLabel
	}
	return packetTypeString[packetType]
}

func normalizeMQTTQoS(qos byte) string {
	switch qos {
	case 0, 1, 2:
		return strconv.Itoa(int(qos))
	default:
		return "0"
	}
}

func normalizeMQTTDirection(direction string) string {
	switch direction {
	case "in", "out":
		return direction
	default:
		return unknownLabel
	}
}

func normalizeMQTTDropReason(reason string) string {
	switch reason {
	case "channel_full", "packet_too_large", "queue_full":
		return reason
	default:
		return unknownLabel
	}
}
