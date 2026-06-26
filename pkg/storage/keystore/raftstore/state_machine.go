package raftstore

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/BAN1ce/skyTree/pkg/cluster/dbpb"
	"github.com/lni/dragonboat/v3/statemachine"
	"google.golang.org/protobuf/proto"
)

var (
	DefaultRequestTimeout = 5 * time.Second
)

type StateMachine struct {
	Handler *StateHandler
}

func NewStateMachine(keyStore store.KeyStoreWithBackup) *StateMachine {
	return &StateMachine{
		Handler: NewStateHandler(keyStore),
	}
}

func (s *StateMachine) Update(bytes []byte) (statemachine.Result, error) {
	var (
		req         = protoDBRequestPool.Get()
		result      = statemachine.Result{}
		ctx, cancel = context.WithTimeout(context.Background(), DefaultRequestTimeout)
		err         error
	)

	defer func() {
		cancel()
		protoDBRequestPool.Put(req)
	}()

	if err = proto.Unmarshal(bytes, req); err != nil {
		return result, err
	}

	result, err = s.Handler.HandleWrite(ctx, req)
	return result, err
}

func (s *StateMachine) Lookup(i interface{}) (interface{}, error) {
	var (
		req, ok     = i.(*dbpb.Request)
		ctx, cancel = context.WithTimeout(context.Background(), DefaultRequestTimeout)
	)
	defer cancel()

	if !ok {
		logger.Logger.Error().Msg(fmt.Sprintf("invalid request type %T", i))
		return nil, fmt.Errorf(`invalid request type %T`, i)
	}

	return s.Handler.HandleRead(ctx, req)

}

func (s *StateMachine) SaveSnapshot(writer io.Writer, collection statemachine.ISnapshotFileCollection, i <-chan struct{}) error {
	return s.Handler.SaveSnapshot(writer, collection, i)
}

func (s *StateMachine) RecoverFromSnapshot(reader io.Reader, files []statemachine.SnapshotFile, i <-chan struct{}) error {
	return s.Handler.RecoverFromSnapshot(reader, files, i)
}

func (s *StateMachine) Close() error {
	return s.Handler.Close()
}
