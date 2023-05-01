package pkg

import "github.com/eclipse/paho.golang/packets"

type SubTree interface {
	CreateSub(clientID string, topics map[string]packets.SubOptions)
	DeleteSub(clientID string, topics []string)
	Match(topic string) (clientIDQos map[string]int32)
	DeleteClient(clientID string)
}
