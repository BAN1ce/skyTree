package state

import (
	"github.com/BAN1ce/skyTree/pkg/proto/system"
	"github.com/lni/dragonboat/v3/statemachine"
	"io"
)

type State struct {
	system.SystemState
}

func (s State) Update(bytes []byte) (statemachine.Result, error) {
	// TODO implement me
	panic("implement me")
}

func (s State) Lookup(i interface{}) (interface{}, error) {
	// TODO implement me
	panic("implement me")
}

func (s State) SaveSnapshot(writer io.Writer, collection statemachine.ISnapshotFileCollection, i <-chan struct{}) error {
	// TODO implement me
	panic("implement me")
}

func (s State) RecoverFromSnapshot(reader io.Reader, files []statemachine.SnapshotFile, i <-chan struct{}) error {
	// TODO implement me
	panic("implement me")
}

func (s State) Close() error {
	// TODO implement me
	panic("implement me")
}
