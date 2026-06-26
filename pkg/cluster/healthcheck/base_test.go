package healthcheck

import (
	"errors"
	"io"
	"testing"

	"github.com/lni/dragonboat/v3/statemachine"
)

type failingLookupStateMachine struct{}

func (f failingLookupStateMachine) Update([]byte) (statemachine.Result, error) {
	return statemachine.Result{}, nil
}

func (f failingLookupStateMachine) Lookup(interface{}) (interface{}, error) {
	return nil, errors.New("lookup delegated")
}

func (f failingLookupStateMachine) SaveSnapshot(io.Writer, statemachine.ISnapshotFileCollection, <-chan struct{}) error {
	return nil
}

func (f failingLookupStateMachine) RecoverFromSnapshot(io.Reader, []statemachine.SnapshotFile, <-chan struct{}) error {
	return nil
}

func (f failingLookupStateMachine) Close() error {
	return nil
}

func TestHealthCheckWrapperHandlesReadOnlyHealthCheck(t *testing.T) {
	wrapper := NewHealthCheckWrapper(failingLookupStateMachine{})

	if _, err := wrapper.Lookup(HealthCheckMessage); err != nil {
		t.Fatalf("Lookup health check returned error: %v", err)
	}
}
