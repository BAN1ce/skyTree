package walsm

import (
	"errors"
	"io"
	"testing"

	"github.com/lni/dragonboat/v3/statemachine"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

type scriptedStateMachine struct {
	applied     [][]byte
	validateErr error
	err         error
}

func (s *scriptedStateMachine) ValidateUpdate([]byte) error {
	return s.validateErr
}

func (s *scriptedStateMachine) Update(b []byte) (statemachine.Result, error) {
	if s.err != nil {
		return statemachine.Result{}, s.err
	}
	copied := append([]byte(nil), b...)
	s.applied = append(s.applied, copied)
	return statemachine.Result{}, nil
}

func (s *scriptedStateMachine) Lookup(interface{}) (interface{}, error) {
	return nil, nil
}

func (s *scriptedStateMachine) SaveSnapshot(io.Writer, statemachine.ISnapshotFileCollection, <-chan struct{}) error {
	return nil
}

func (s *scriptedStateMachine) RecoverFromSnapshot(io.Reader, []statemachine.SnapshotFile, <-chan struct{}) error {
	return nil
}

type scriptedWALStore struct {
	saveCalls int
	saveErrOn int
	saveErr   error
}

func (s *scriptedWALStore) Save(uint64, []raftpb.Entry) error {
	s.saveCalls++
	if s.saveCalls == s.saveErrOn {
		return s.saveErr
	}
	return nil
}

func (s *scriptedWALStore) SaveSnapshot(SnapshotMeta) error {
	return nil
}

func (s *scriptedWALStore) ReleaseLockTo(uint64) error {
	return nil
}

func (s *scriptedWALStore) Close() error {
	return nil
}

func TestEngineSkipsFailedUncommittedEntryOnRestart(t *testing.T) {
	dir := t.TempDir()
	poisonErr := errors.New("poison update")
	firstSM := &scriptedStateMachine{validateErr: poisonErr}
	engine, err := NewEngine(firstSM, Options{BaseDir: dir})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	if _, err := engine.Write([]byte("bad-entry")); !errors.Is(err, poisonErr) {
		t.Fatalf("Write error = %v, want %v", err, poisonErr)
	}
	if err := engine.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	secondSM := &scriptedStateMachine{}
	restarted, err := NewEngine(secondSM, Options{BaseDir: dir})
	if err != nil {
		t.Fatalf("NewEngine after failed write: %v", err)
	}
	defer restarted.Close()

	if len(secondSM.applied) != 0 {
		t.Fatalf("replayed failed entry, applied=%q", secondSM.applied)
	}
}

func TestEngineCommitSaveFailureDoesNotApplyUpdate(t *testing.T) {
	commitErr := errors.New("commit save failed")
	sm := &scriptedStateMachine{}
	engine := &Engine{
		sm:        sm,
		wal:       &scriptedWALStore{saveErrOn: 2, saveErr: commitErr},
		nextIndex: 1,
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}

	if _, err := engine.Write([]byte("committed-only-after-save")); !errors.Is(err, commitErr) {
		t.Fatalf("Write error = %v, want %v", err, commitErr)
	}
	if len(sm.applied) != 0 {
		t.Fatalf("state machine was applied before commit persisted: %q", sm.applied)
	}
}
