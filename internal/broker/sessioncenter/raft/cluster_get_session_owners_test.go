package raft

import (
	"context"
	"testing"

	"github.com/BAN1ce/skyTree/proto/proto_session"
	"github.com/lni/dragonboat/v3/statemachine"
)

type fakeSessionClusterClient struct {
	readResult interface{}
	readErr    error
	lastQuery  interface{}
}

func (f *fakeSessionClusterClient) Write(context.Context, []byte) (statemachine.Result, error) {
	return statemachine.Result{}, nil
}

func (f *fakeSessionClusterClient) Read(_ context.Context, query interface{}) (interface{}, error) {
	f.lastQuery = query
	return f.readResult, f.readErr
}

func (f *fakeSessionClusterClient) GetNodeID() uint64 {
	return 1
}

func TestClusterGetSessionOwners(t *testing.T) {
	client := &fakeSessionClusterClient{
		readResult: &proto_session.ReadSessionOwnersResponse{
			Items: []*proto_session.ReadSessionOwnerItem{{
				ClientID: "c1",
				Exist:    true,
				Owner:    &proto_session.SessionOwner{ClientID: "c1", NodeID: 1, Online: true},
			}},
		},
	}
	cluster := NewCluster(1, client)

	resp, err := cluster.GetSessionOwners(context.Background(), &proto_session.ReadSessionOwnersRequest{ClientIDs: []string{"c1"}})
	if err != nil {
		t.Fatalf("get session owners: %v", err)
	}
	if resp == nil || len(resp.GetItems()) != 1 || resp.GetItems()[0].GetClientID() != "c1" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if _, ok := client.lastQuery.(*proto_session.ReadSessionOwnersRequest); !ok {
		t.Fatalf("unexpected query type %T", client.lastQuery)
	}
}
