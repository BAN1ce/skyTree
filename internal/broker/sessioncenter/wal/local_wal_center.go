package wal

import (
	"context"
	"fmt"
	"time"

	"github.com/BAN1ce/skyTree/internal/broker/sessioncenter/statemachine"
	"github.com/BAN1ce/skyTree/internal/localstate/walsm"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/session"
	"github.com/BAN1ce/skyTree/proto/proto_session"
	"google.golang.org/protobuf/proto"
)

// LocalWALCenter is a single-node persistent implementation of session.Center.
// It writes every state update through an etcd WAL and periodically snapshots the full state.
type LocalWALCenter struct {
	engine *walsm.Engine
}

var _ session.Center = (*LocalWALCenter)(nil)

func NewLocalWALCenter(baseDir string, snapshotInterval time.Duration, snapshotEntries uint64) (*LocalWALCenter, error) {
	sm := statemachine.NewStateMachine()
	engine, err := walsm.NewEngine(sm, walsm.Options{
		Name:             "session_center",
		BaseDir:          baseDir,
		SnapshotEntries:  snapshotEntries,
		SnapshotInterval: snapshotInterval,
	})
	if err != nil {
		return nil, err
	}
	return &LocalWALCenter{engine: engine}, nil
}

func (c *LocalWALCenter) Close() error {
	if c.engine == nil {
		return nil
	}
	return c.engine.Close()
}

func (c *LocalWALCenter) OpenSessionForConnect(ctx context.Context, request *proto_session.OpenSessionForConnectRequest) (*proto_session.OpenSessionForConnectResponse, error) {
	if request == nil {
		return nil, fmt.Errorf("nil open session request")
	}
	updateBytes, err := statemachine.EncodeRequest(proto_session.SessionRequestType_OPEN_SESSION_FOR_CONNECT, request.GetClientID(), 0, request)
	if err != nil {
		return nil, err
	}
	res, err := c.engine.Write(updateBytes)
	if err != nil {
		return nil, err
	}
	resp := &proto_session.OpenSessionForConnectResponse{}
	if err := proto.Unmarshal(res.Data, resp); err != nil {
		return nil, err
	}
	_ = ctx
	return resp, nil
}

func (c *LocalWALCenter) TakeOverSessionOwner(ctx context.Context, request *proto_session.TakeOverSessionOwnerRequest) (*proto_session.TakeOverSessionOwnerResponse, error) {
	if request == nil || request.GetOwner() == nil {
		return nil, fmt.Errorf("nil take over owner request")
	}
	updateBytes, err := statemachine.EncodeRequest(proto_session.SessionRequestType_TAKE_OVER_SESSION_OWNER, request.GetOwner().GetClientID(), 0, request)
	if err != nil {
		return nil, err
	}
	res, err := c.engine.Write(updateBytes)
	if err != nil {
		return nil, err
	}
	resp := &proto_session.TakeOverSessionOwnerResponse{}
	if err := proto.Unmarshal(res.Data, resp); err != nil {
		return nil, err
	}
	_ = ctx
	return resp, nil
}

func (c *LocalWALCenter) ReplaceSessionStateOnCleanStart(ctx context.Context, request *proto_session.ReplaceSessionStateOnCleanStartRequest) error {
	updateBytes, err := statemachine.EncodeRequest(proto_session.SessionRequestType_REPLACE_SESSION_STATE_ON_CLEAN_START, request.GetClientID(), 0, request)
	if err != nil {
		return err
	}
	_, err = c.engine.Write(updateBytes)
	_ = ctx
	return err
}

func (c *LocalWALCenter) SaveOfflineState(ctx context.Context, request *proto_session.SaveOfflineStateRequest) error {
	updateBytes, err := statemachine.EncodeRequest(proto_session.SessionRequestType_SAVE_OFFLINE_STATE, request.GetClientID(), 0, request)
	if err != nil {
		return err
	}
	_, err = c.engine.Write(updateBytes)
	_ = ctx
	return err
}

func (c *LocalWALCenter) RemoveOutgoingUnfinished(ctx context.Context, request *proto_session.RemoveOutgoingUnfinishedRequest) error {
	updateBytes, err := statemachine.EncodeRequest(proto_session.SessionRequestType_REMOVE_OUTGOING_UNFINISHED, request.GetClientID(), 0, request)
	if err != nil {
		return err
	}
	_, err = c.engine.Write(updateBytes)
	_ = ctx
	return err
}

func (c *LocalWALCenter) UpsertIncomingUnfinished(ctx context.Context, request *proto_session.UpsertIncomingUnfinishedRequest) error {
	updateBytes, err := statemachine.EncodeRequest(proto_session.SessionRequestType_UPSERT_INCOMING_UNFINISHED, request.GetClientID(), 0, request)
	if err != nil {
		return err
	}
	_, err = c.engine.Write(updateBytes)
	_ = ctx
	return err
}

func (c *LocalWALCenter) RemoveIncomingUnfinished(ctx context.Context, request *proto_session.RemoveIncomingUnfinishedRequest) error {
	updateBytes, err := statemachine.EncodeRequest(proto_session.SessionRequestType_REMOVE_INCOMING_UNFINISHED, request.GetClientID(), 0, request)
	if err != nil {
		return err
	}
	_, err = c.engine.Write(updateBytes)
	_ = ctx
	return err
}

func (c *LocalWALCenter) CommitOutgoingProgress(ctx context.Context, request *proto_session.CommitOutgoingProgressRequest) error {
	updateBytes, err := statemachine.EncodeRequest(proto_session.SessionRequestType_COMMIT_OUTGOING_PROGRESS, request.GetClientID(), 0, request)
	if err != nil {
		return err
	}
	_, err = c.engine.Write(updateBytes)
	_ = ctx
	return err
}

func (c *LocalWALCenter) DeleteSession(ctx context.Context, request *proto_session.DeleteSessionRequest) error {
	updateBytes, err := statemachine.EncodeRequest(proto_session.SessionRequestType_DELETE_SESSION, request.GetClientID(), 0, request)
	if err != nil {
		return err
	}
	_, err = c.engine.Write(updateBytes)
	_ = ctx
	return err
}

func (c *LocalWALCenter) DeleteExpiredSessions(ctx context.Context, nowUnixNano int64) ([]string, error) {
	request := &proto_session.DeleteExpiredSessionsRequest{NowUnixNano: nowUnixNano}
	updateBytes, err := statemachine.EncodeRequest(proto_session.SessionRequestType_DELETE_EXPIRED_SESSIONS, "", 0, request)
	if err != nil {
		return nil, err
	}
	res, err := c.engine.Write(updateBytes)
	if err != nil {
		return nil, err
	}
	resp := &proto_session.DeleteExpiredSessionsResponse{}
	if err := proto.Unmarshal(res.Data, resp); err != nil {
		return nil, err
	}
	_ = ctx
	return resp.GetClientIDs(), nil
}

func (c *LocalWALCenter) GetSession(ctx context.Context, request *proto_session.ReadSessionRequest) (*proto_session.ReadSessionResponse, error) {
	out, err := c.engine.Read(request)
	if err != nil {
		return nil, err
	}
	resp, ok := out.(*proto_session.ReadSessionResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T", out)
	}
	_ = ctx
	return resp, nil
}

func (c *LocalWALCenter) GetSessionOwner(ctx context.Context, request *proto_session.ReadSessionOwnerRequest) (*proto_session.ReadSessionOwnerResponse, error) {
	out, err := c.engine.Read(request)
	if err != nil {
		return nil, err
	}
	resp, ok := out.(*proto_session.ReadSessionOwnerResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T", out)
	}
	_ = ctx
	return resp, nil
}

func (c *LocalWALCenter) GetSessionOwners(ctx context.Context, request *proto_session.ReadSessionOwnersRequest) (*proto_session.ReadSessionOwnersResponse, error) {
	out, err := c.engine.Read(request)
	if err != nil {
		return nil, err
	}
	resp, ok := out.(*proto_session.ReadSessionOwnersResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T", out)
	}
	_ = ctx
	return resp, nil
}
