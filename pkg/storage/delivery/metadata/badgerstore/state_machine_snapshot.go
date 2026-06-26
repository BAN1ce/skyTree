package badgerstore

import (
	"context"
	"io"

	"github.com/dgraph-io/badger"
	"github.com/lni/dragonboat/v3/statemachine"
)

func (s *DeliveryStateMachine) SaveSnapshot(writer io.Writer, _ statemachine.ISnapshotFileCollection, _ <-chan struct{}) error {
	if s.db == nil {
		return nil
	}
	_, err := s.db.Backup(writer, 0)
	return err
}

func (s *DeliveryStateMachine) RecoverFromSnapshot(reader io.Reader, _ []statemachine.SnapshotFile, _ <-chan struct{}) error {
	if s.db == nil {
		return nil
	}
	return s.db.Load(reader, 0)
}

func (s *DeliveryStateMachine) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// helper to open badger for a shard.
func openShardBadger(path string) (*badger.DB, error) {
	opt := badger.DefaultOptions(path)
	opt.SyncWrites = false
	return badger.Open(opt)
}

// local-only helper; not used by the Raft client directly but useful for tests.
func (s *DeliveryStateMachine) ensureSchema(_ context.Context) error {
	return nil
}
