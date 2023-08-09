package subtree

import (
	"context"
	"github.com/BAN1ce/Tree/app"
	"github.com/BAN1ce/Tree/proto"
	"github.com/BAN1ce/Tree/state/api"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/eclipse/paho.golang/packets"
	"go.uber.org/zap"
	"time"
)

type SubTree struct {
	app *app.App
}

func NewSubTree(app2 *app.App) *SubTree {
	return &SubTree{app: app2}
}

func (s *SubTree) CreateSub(clientID string, topics map[string]packets.SubOptions) {
	var (
		ctx, cancel = context.WithTimeout(context.TODO(), 5*time.Second)
		req         = proto.SubRequest{
			Topics:   make(map[string]*proto.SubInfo),
			ClientID: clientID,
		}
	)
	for topic, option := range topics {
		req.Topics[topic] = &proto.SubInfo{
			Retain: option.RetainAsPublished,
			QoS:    int32(option.QoS),
		}
	}
	defer cancel()
	if _, err := s.app.Subscribe(ctx, &req); err != nil {
		logger.Logger.Error("subscribe failed", zap.String("clientID", clientID), zap.Error(err))
	}
	// TODO
}

func (s *SubTree) DeleteSub(clientID string, topics []string) {
	s.app.UnSubscribe(context.TODO(), &proto.UnSubRequest{
		ClientID: clientID,
		Topics:   topics,
	})
}

func (s *SubTree) Match(topic string) (clientIDQos map[string]int32) {
	var (
		ctx, cancel = context.WithTimeout(context.TODO(), 5*time.Second)
	)
	defer cancel()
	rsp, err := s.app.MatchTopic(ctx, &api.MatchTopicRequest{
		Topic: topic,
	})
	if err != nil {
		logger.Logger.Error("match store failed", zap.String("store", topic), zap.Error(err))
		return
	}
	clientIDQos = map[string]int32{}
	for _, client := range rsp.Client {
		clientIDQos[client.ClientID] = client.QoS
	}
	return
}

func (s *SubTree) DeleteClient(clientID string) {
	//TODO implement me
	panic("implement me")
}
