package store

import (
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/packet"
	"github.com/BAN1ce/skyTree/pkg/proto"
	"github.com/BAN1ce/skyTree/pkg/util"
	proto2 "github.com/golang/protobuf/proto"
	"github.com/lni/dragonboat/statemachine"
	"io"
	"sync"
)

type ClientID = string

type CoreModel struct {
	model *proto.CoreModel
	mux   sync.RWMutex
}

func (m *CoreModel) ReadOrStoreClient(model *proto.ClientModel) (old *proto.ClientModel, exists bool) {
	m.mux.Lock()
	defer m.mux.Unlock()
	return m.readOrStoreClient(model)
}

func (m *CoreModel) readOrStoreClient(model *proto.ClientModel) (old *proto.ClientModel, exists bool) {
	if model.GetID() == "" {
		return model, exists
	}
	if old, exists = m.model.Client[model.GetID()]; exists {
		m.model.Client[model.GetID()] = model
		return old, exists
	}
	m.model.Client[model.GetID()] = model
	return model, exists
}

func (m *CoreModel) DeleteSub() {

}

func (m *CoreModel) CreateClientSubscription(clientID string, topics map[string]int32, meta map[string]string) {
	for topic, qos := range topics {
		if !util.HasWildcard(topic) {
			m.createHashSub(clientID, topic, qos)
		}
	}
	return
}

func (m *CoreModel) createHashSub(clientID, topic string, qos int32) {
	var (
		clientModel *proto.ClientModel
		ok          bool
	)
	if clientModel, ok = m.model.Client[clientID]; !ok {
		clientModel = newClientModel(clientID)
		m.model.Client[clientID] = clientModel
	}
	clientModel.SubTopic[topic] = qos
	return
}

func (m *CoreModel) DeleteClientSubscription(clientID string) error {
	return nil
}

func (m *CoreModel) getSubscriptionTree(topic string) (*proto.TreeNode, error) {
	var (
		node = new(proto.TreeNode)
	)
	return node, nil
}

func (m *CoreModel) getHashSubTopic(topic string) (clients []*proto.ClientModel, err error) {
	if tmp, ok := m.model.Hash.HashSubTopic[topic]; ok {
		for _, v := range tmp.GetClient() {
			clients = append(clients, v)
		}
	} else {
		logger.Logger.Warn("topic not found: ", topic)
	}
	return
}

func (m *CoreModel) ReadTopicSubscribers(topic string) ([]*proto.ClientModel, error) {
	m.mux.Lock()
	defer m.mux.Unlock()
	return m.getHashSubTopic(topic)
}

func (m *CoreModel) Update(bytes []byte) uint64 {
	var (
		command = DecodeCommand(bytes)
	)
	switch command.commandType {
	case packet.SUBSCRIBE:
		req := command.ToSub()
		m.CreateClientSubscription(req.GetClientID(), req.GetSubTopic(), req.GetMeta())
	default:
		logger.Logger.Error("unknown command type: ", command.commandType)
	}
	return 0
}

func (m *CoreModel) Lookup(bytes []byte) []byte {
	// TODO implement me
	panic("implement me")
}

func (m *CoreModel) SaveSnapshot(writer io.Writer, collection statemachine.ISnapshotFileCollection, i <-chan struct{}) (uint64, error) {
	m.mux.RLock()
	defer m.mux.RUnlock()

	if data, err := proto2.Marshal(m.model); err != nil {
		return 0, err
	} else {
		l, err := writer.Write(data)
		return uint64(l), err
	}
}

func (m *CoreModel) RecoverFromSnapshot(reader io.Reader, files []statemachine.SnapshotFile, i <-chan struct{}) error {
	return nil
}

func (m *CoreModel) Close() {
}

func (m *CoreModel) GetHash() uint64 {
	return 0
}
