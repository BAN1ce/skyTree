package broker

import (
	"github.com/eclipse/paho.golang/packets"
)

type QoS = byte

const (
	QoS0 = QoS(0)
	QoS1 = 1
	QoS2 = 2
)

// SubClient is the interface of the subscription client.
type SubClient interface {
	GetClientID() string
	GetQoS() int32
}

// SubCenter is the interface of the subscription center.
// It is used to manage the subscription of the clients.
type SubCenter interface {
	CreateSub(clientID string, topics map[string]packets.SubOptions) error
	DeleteSub(clientID string, topics []string) error
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
