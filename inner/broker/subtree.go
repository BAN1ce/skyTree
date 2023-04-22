package broker

type SubTree interface {
	CreateSub(clientID string, topics map[string]int32)
	DeleteSub(clientID string, topics []string)
	Match(topic string) (clientIDQos map[string]int32)
	DeleteClient(clientID string)
}
