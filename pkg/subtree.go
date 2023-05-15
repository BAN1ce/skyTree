package pkg

import "github.com/eclipse/paho.golang/packets"

const (
	QoS0 = 0
	QoS1 = 1
	QoS2 = 2
)

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
