package memory

import (
	"context"
	"testing"

	"github.com/BAN1ce/skyTree/proto/proto_session"
)

func TestCoreGetSessionOwnersEmptyRequest(t *testing.T) {
	core := NewCore()
	resp, err := core.GetSessionOwners(context.Background(), &proto_session.ReadSessionOwnersRequest{})
	if err != nil {
		t.Fatalf("get session owners: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if len(resp.GetItems()) != 0 {
		t.Fatalf("expected empty items, got %d", len(resp.GetItems()))
	}
}

func TestCoreGetSessionOwnersDedupAndMixedExistence(t *testing.T) {
	core := NewCore()
	ctx := context.Background()
	_, err := core.TakeOverSessionOwner(ctx, &proto_session.TakeOverSessionOwnerRequest{Owner: &proto_session.SessionOwner{ClientID: "c1", NodeID: 1, Online: true}})
	if err != nil {
		t.Fatalf("take over c1: %v", err)
	}
	_, err = core.TakeOverSessionOwner(ctx, &proto_session.TakeOverSessionOwnerRequest{Owner: &proto_session.SessionOwner{ClientID: "c2", NodeID: 2, Online: false}})
	if err != nil {
		t.Fatalf("take over c2: %v", err)
	}

	resp, err := core.GetSessionOwners(ctx, &proto_session.ReadSessionOwnersRequest{ClientIDs: []string{"c1", "c1", "missing", "c2"}})
	if err != nil {
		t.Fatalf("get session owners: %v", err)
	}
	items := resp.GetItems()
	if len(items) != 3 {
		t.Fatalf("expected 3 items after dedupe, got %d", len(items))
	}
	if items[0].GetClientID() != "c1" || !items[0].GetExist() || items[0].GetOwner() == nil || items[0].GetOwner().GetNodeID() != 1 {
		t.Fatalf("unexpected first item: %+v", items[0])
	}
	if items[1].GetClientID() != "missing" || items[1].GetExist() || items[1].GetOwner() != nil {
		t.Fatalf("unexpected second item: %+v", items[1])
	}
	if items[2].GetClientID() != "c2" || !items[2].GetExist() || items[2].GetOwner() == nil || items[2].GetOwner().GetNodeID() != 2 {
		t.Fatalf("unexpected third item: %+v", items[2])
	}
}

func TestCoreGetSessionOwnersReturnsClonedOwner(t *testing.T) {
	core := NewCore()
	ctx := context.Background()
	_, err := core.TakeOverSessionOwner(ctx, &proto_session.TakeOverSessionOwnerRequest{Owner: &proto_session.SessionOwner{ClientID: "c1", NodeID: 7, Online: true}})
	if err != nil {
		t.Fatalf("take over c1: %v", err)
	}

	resp, err := core.GetSessionOwners(ctx, &proto_session.ReadSessionOwnersRequest{ClientIDs: []string{"c1"}})
	if err != nil {
		t.Fatalf("get session owners #1: %v", err)
	}
	resp.GetItems()[0].Owner.NodeID = 99

	resp2, err := core.GetSessionOwners(ctx, &proto_session.ReadSessionOwnersRequest{ClientIDs: []string{"c1"}})
	if err != nil {
		t.Fatalf("get session owners #2: %v", err)
	}
	if got := resp2.GetItems()[0].GetOwner().GetNodeID(); got != 7 {
		t.Fatalf("expected cloned owner node id 7, got %d", got)
	}
}
