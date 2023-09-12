package broker

import (
	"github.com/BAN1ce/Tree/proto"
	"github.com/BAN1ce/Tree/state/api"
	"github.com/BAN1ce/Tree/state/store"
	"github.com/eclipse/paho.golang/packets"
)

type LocalSubCenter struct {
	state *store.State
}

func NewLocalSubCenter() *LocalSubCenter {
	return &LocalSubCenter{
		state: store.NewState(),
	}
}
func (l *LocalSubCenter) CreateSub(clientID string, topics map[string]packets.SubOptions) error {
	var topicsRequest = make(map[string]*proto.SubInfo)
	for topic, opt := range topics {
		topicsRequest[topic] = &proto.SubInfo{
			QoS: int32(opt.QoS),
		}
	}

	_, err := l.state.HandleSubRequest(&proto.SubRequest{
		Topics:   topicsRequest,
		ClientID: clientID,
	})
	return err
}

func (l *LocalSubCenter) DeleteSub(clientID string, topics []string) error {
	_, err := l.state.HandleUnSubRequest(&proto.UnSubRequest{
		Topics:   topics,
		ClientID: clientID,
	})
	return err
}

func (l *LocalSubCenter) Match(topic string) (clientIDQos map[string]int32) {
	rsp := l.state.MatchTopic(&api.MatchTopicRequest{
		Topic: topic,
	})

	clientIDQos = map[string]int32{}
	for _, client := range rsp.Client {
		clientIDQos[client.ClientID] = client.QoS
	}
	return
}

func (l *LocalSubCenter) DeleteClient(clientID string) {
	//TODO implement me
	panic("implement me")
}
