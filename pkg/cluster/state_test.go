package cluster

import (
	"context"
	"errors"
	"testing"
)

type stateTestHashStore struct {
	hgetAllFn func(ctx context.Context, key []byte) (map[string]string, error)
}

func (s *stateTestHashStore) HSet(context.Context, []byte, [][]byte) error { return nil }

func (s *stateTestHashStore) HGet(context.Context, []byte, []byte) ([]byte, bool, error) {
	return nil, false, nil
}

func (s *stateTestHashStore) HDel(context.Context, []byte, [][]byte) error { return nil }

func (s *stateTestHashStore) HGetAll(ctx context.Context, key []byte) (map[string]string, error) {
	if s.hgetAllFn == nil {
		return map[string]string{}, nil
	}
	return s.hgetAllFn(ctx, key)
}

func (s *stateTestHashStore) HPrefix(context.Context, []byte, []byte) (map[string]string, error) {
	return map[string]string{}, nil
}

func (s *stateTestHashStore) DeleteHash(context.Context, []byte) error { return nil }

func TestStateImplListNodePropagatesHGetAllError(t *testing.T) {
	wantErr := errors.New("hgetall failed")
	state := NewStateImpl(&stateTestHashStore{
		hgetAllFn: func(context.Context, []byte) (map[string]string, error) {
			return nil, wantErr
		},
	})

	nodes, err := state.ListNode(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("ListNode() error = %v, want %v", err, wantErr)
	}
	if nodes != nil {
		t.Fatalf("ListNode() nodes = %v, want nil on error", nodes)
	}
}
