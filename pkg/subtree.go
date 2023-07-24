package pkg

import "github.com/eclipse/paho.golang/packets"

type QoS = byte

const (
	QoS0 = QoS(0)
	QoS1 = 1
	QoS2 = 2
)

func IsQoS0(qos byte) bool {
	return qos == QoS0
}

func IsQoS1(qos byte) bool {
	return qos == QoS1
}

func IsQos2(qos byte) bool {
	return qos == QoS2
}

type SubClient interface {
	GetClientID() string
	GetQoS() int32
}
type SubTree interface {
	CreateSub(clientID string, topics map[string]packets.SubOptions)
	DeleteSub(clientID string, topics []string)
	Match(topic string) (clientIDQos map[string]int32)
	DeleteClient(clientID string)
}

func Int32ToQoS(qos int32) QoS {
	switch qos {
	case 1:
		return QoS1
	case 2:
		return QoS2
	case 0:
		return QoS0
	default:
		return QoS0
	}
}
