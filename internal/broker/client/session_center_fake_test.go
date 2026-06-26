package client

import (
	"context"

	"github.com/BAN1ce/skyTree/pkg/brokerapi/session"
	"github.com/BAN1ce/skyTree/proto/proto_session"
)

type fakeSessionCenter struct{}

var _ session.Center = (*fakeSessionCenter)(nil)

func (f *fakeSessionCenter) OpenSessionForConnect(ctx context.Context, request *proto_session.OpenSessionForConnectRequest) (*proto_session.OpenSessionForConnectResponse, error) {
	return &proto_session.OpenSessionForConnectResponse{}, nil
}

func (f *fakeSessionCenter) TakeOverSessionOwner(ctx context.Context, request *proto_session.TakeOverSessionOwnerRequest) (*proto_session.TakeOverSessionOwnerResponse, error) {
	return &proto_session.TakeOverSessionOwnerResponse{
		Owner: request.GetOwner(),
	}, nil
}

func (f *fakeSessionCenter) ReplaceSessionStateOnCleanStart(ctx context.Context, request *proto_session.ReplaceSessionStateOnCleanStartRequest) error {
	return nil
}

func (f *fakeSessionCenter) SaveOfflineState(ctx context.Context, request *proto_session.SaveOfflineStateRequest) error {
	return nil
}

func (f *fakeSessionCenter) RemoveOutgoingUnfinished(ctx context.Context, request *proto_session.RemoveOutgoingUnfinishedRequest) error {
	return nil
}

func (f *fakeSessionCenter) UpsertIncomingUnfinished(ctx context.Context, request *proto_session.UpsertIncomingUnfinishedRequest) error {
	return nil
}

func (f *fakeSessionCenter) RemoveIncomingUnfinished(ctx context.Context, request *proto_session.RemoveIncomingUnfinishedRequest) error {
	return nil
}

func (f *fakeSessionCenter) CommitOutgoingProgress(ctx context.Context, request *proto_session.CommitOutgoingProgressRequest) error {
	return nil
}

func (f *fakeSessionCenter) DeleteSession(ctx context.Context, request *proto_session.DeleteSessionRequest) error {
	return nil
}

func (f *fakeSessionCenter) GetSession(ctx context.Context, request *proto_session.ReadSessionRequest) (*proto_session.ReadSessionResponse, error) {
	return &proto_session.ReadSessionResponse{Exist: false}, nil
}

func (f *fakeSessionCenter) GetSessionOwner(ctx context.Context, request *proto_session.ReadSessionOwnerRequest) (*proto_session.ReadSessionOwnerResponse, error) {
	return &proto_session.ReadSessionOwnerResponse{Exist: false}, nil
}

func (f *fakeSessionCenter) GetSessionOwners(ctx context.Context, request *proto_session.ReadSessionOwnersRequest) (*proto_session.ReadSessionOwnersResponse, error) {
	return &proto_session.ReadSessionOwnersResponse{Items: []*proto_session.ReadSessionOwnerItem{}}, nil
}
