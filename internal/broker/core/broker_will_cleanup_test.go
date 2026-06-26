package core

import (
	"context"
	"testing"

	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/session"
	"github.com/BAN1ce/skyTree/proto/proto_session"
)

type willCleanupSessionCenter struct {
	sessionResp *proto_session.ReadSessionResponse
	saveReq     *proto_session.SaveOfflineStateRequest
}

var _ session.Center = (*willCleanupSessionCenter)(nil)

func (c *willCleanupSessionCenter) OpenSessionForConnect(context.Context, *proto_session.OpenSessionForConnectRequest) (*proto_session.OpenSessionForConnectResponse, error) {
	return nil, nil
}

func (c *willCleanupSessionCenter) TakeOverSessionOwner(context.Context, *proto_session.TakeOverSessionOwnerRequest) (*proto_session.TakeOverSessionOwnerResponse, error) {
	return nil, nil
}

func (c *willCleanupSessionCenter) ReplaceSessionStateOnCleanStart(context.Context, *proto_session.ReplaceSessionStateOnCleanStartRequest) error {
	return nil
}

func (c *willCleanupSessionCenter) SaveOfflineState(_ context.Context, request *proto_session.SaveOfflineStateRequest) error {
	c.saveReq = request
	return nil
}

func (c *willCleanupSessionCenter) RemoveOutgoingUnfinished(context.Context, *proto_session.RemoveOutgoingUnfinishedRequest) error {
	return nil
}

func (c *willCleanupSessionCenter) UpsertIncomingUnfinished(context.Context, *proto_session.UpsertIncomingUnfinishedRequest) error {
	return nil
}

func (c *willCleanupSessionCenter) RemoveIncomingUnfinished(context.Context, *proto_session.RemoveIncomingUnfinishedRequest) error {
	return nil
}

func (c *willCleanupSessionCenter) CommitOutgoingProgress(context.Context, *proto_session.CommitOutgoingProgressRequest) error {
	return nil
}

func (c *willCleanupSessionCenter) DeleteSession(context.Context, *proto_session.DeleteSessionRequest) error {
	return nil
}

func (c *willCleanupSessionCenter) GetSession(context.Context, *proto_session.ReadSessionRequest) (*proto_session.ReadSessionResponse, error) {
	return c.sessionResp, nil
}

func (c *willCleanupSessionCenter) GetSessionOwner(context.Context, *proto_session.ReadSessionOwnerRequest) (*proto_session.ReadSessionOwnerResponse, error) {
	return nil, nil
}

func (c *willCleanupSessionCenter) GetSessionOwners(context.Context, *proto_session.ReadSessionOwnersRequest) (*proto_session.ReadSessionOwnersResponse, error) {
	return &proto_session.ReadSessionOwnersResponse{Items: []*proto_session.ReadSessionOwnerItem{}}, nil
}

func TestClearPublishedWillFromSession(t *testing.T) {
	center := &willCleanupSessionCenter{
		sessionResp: &proto_session.ReadSessionResponse{
			Exist: true,
			Session: &proto_session.Session{
				ClientID:              "client-a",
				SessionExpiryInterval: 30,
				WillMessage: &proto_session.WillMessage{
					Topic: "will/topic",
				},
				UnfinishedMessages: []*proto_session.UnfinishedMessage{
					{MessageID: "msg-1"},
				},
			},
		},
	}
	b := &Broker{
		state: brokerStateCenters{sessionCenter: center},
	}

	b.clearPublishedWillFromSession(context.Background(), &brokerpublish.Message{
		SendClientID: "client-a",
		OwnerToken:   "owner-token",
		Will:         true,
	})

	if center.saveReq == nil {
		t.Fatal("expected SaveOfflineState to be called")
	}
	if !center.saveReq.GetClearWill() {
		t.Fatal("expected ClearWill=true when clearing published will")
	}
	if center.saveReq.GetClientID() != "client-a" {
		t.Fatalf("expected client-a, got %q", center.saveReq.GetClientID())
	}
	if center.saveReq.GetSessionExpiryInterval() != 30 {
		t.Fatalf("expected session expiry 30, got %d", center.saveReq.GetSessionExpiryInterval())
	}
	if center.saveReq.GetOwnerToken() != "owner-token" {
		t.Fatalf("expected owner token to be forwarded, got %q", center.saveReq.GetOwnerToken())
	}
}
