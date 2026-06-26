package cluster

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/BAN1ce/skyTree/logger"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
)

const (
	NodeMetaBucket = store.KeyNamespaceNodeMetaKey
)

type StateImpl struct {
	store store.HashStore
}

func NewStateImpl(hashStore store.HashStore) *StateImpl {
	return &StateImpl{
		store: hashStore,
	}
}

func (s *StateImpl) ListNode(ctx context.Context) ([]*NodeMeta, error) {
	var (
		result, err = s.store.HGetAll(ctx, []byte(NodeMetaBucket))
		nodes       []*NodeMeta
	)

	if err != nil {
		return nil, err
	}

	for _, v := range result {
		var node NodeMeta
		err = json.Unmarshal([]byte(v), &node)
		if err != nil {
			logger.Logger.Error().Err(err).Str("key", string(v)).Msg("failed to unmarshal node")
			return nil, err
		}
		nodes = append(nodes, &node)
	}

	return nodes, nil

}

func (s *StateImpl) AddNode(ctx context.Context, request *NodeMeta) error {
	var data, err = json.Marshal(request)
	if err != nil {
		return err
	}

	return s.store.HSet(ctx, []byte(NodeMetaBucket), [][]byte{[]byte(fmt.Sprintf("%d", request.LocalNodeID)), data})

}

func (s *StateImpl) RemoveNode(ctx context.Context, nodeId uint64) error {
	return s.store.HDel(ctx, []byte(NodeMetaBucket), [][]byte{[]byte(fmt.Sprintf("%d", nodeId))})
}
