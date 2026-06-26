package statemachine

import (
	"strings"
	"testing"

	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/proto/proto_session"
)

func TestUpdateReturnsOpenSessionBusinessError(t *testing.T) {
	logger.LoadForTest()

	sm := NewStateMachine()
	data, err := EncodeRequest(
		proto_session.SessionRequestType_OPEN_SESSION_FOR_CONNECT,
		"",
		1,
		&proto_session.OpenSessionForConnectRequest{},
	)
	if err != nil {
		t.Fatalf("encode request: %v", err)
	}

	_, err = sm.Update(data)
	if err == nil {
		t.Fatal("expected business error for empty client id")
	}
	if !strings.Contains(err.Error(), "client id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateReturnsTakeOverOwnerBusinessError(t *testing.T) {
	logger.LoadForTest()

	sm := NewStateMachine()
	data, err := EncodeRequest(
		proto_session.SessionRequestType_TAKE_OVER_SESSION_OWNER,
		"",
		1,
		&proto_session.TakeOverSessionOwnerRequest{},
	)
	if err != nil {
		t.Fatalf("encode request: %v", err)
	}

	_, err = sm.Update(data)
	if err == nil {
		t.Fatal("expected business error for missing owner")
	}
	if !strings.Contains(err.Error(), "owner") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLookupReadSessionOwners(t *testing.T) {
	logger.LoadForTest()

	sm := NewStateMachine()
	out, err := sm.Lookup(&proto_session.ReadSessionOwnersRequest{ClientIDs: []string{"c1", "c1"}})
	if err != nil {
		t.Fatalf("lookup read session owners: %v", err)
	}
	resp, ok := out.(*proto_session.ReadSessionOwnersResponse)
	if !ok {
		t.Fatalf("unexpected response type %T", out)
	}
	if len(resp.GetItems()) != 1 {
		t.Fatalf("expected one deduped item, got %d", len(resp.GetItems()))
	}
	if resp.GetItems()[0].GetClientID() != "c1" || resp.GetItems()[0].GetExist() {
		t.Fatalf("unexpected owner item: %+v", resp.GetItems()[0])
	}
}
