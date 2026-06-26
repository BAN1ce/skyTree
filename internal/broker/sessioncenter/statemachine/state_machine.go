package statemachine

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/BAN1ce/skyTree/internal/broker/sessioncenter/internal/pool"
	"github.com/BAN1ce/skyTree/internal/broker/sessioncenter/memory"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/proto/proto_session"
	dbsm "github.com/lni/dragonboat/v3/statemachine"
	"google.golang.org/protobuf/proto"
)

const stateMachineRequestTimeout = 5 * time.Second

type StateMachine struct {
	core *memory.Core
}

func NewStateMachine() *StateMachine {
	return &StateMachine{
		core: memory.NewCore(),
	}
}

func (s *StateMachine) ValidateUpdate(bytes []byte) error {
	rawReq := pool.RawRequest.Get().(*proto_session.SessionRequest)
	defer func() {
		rawReq.Reset()
		pool.RawRequest.Put(rawReq)
	}()

	if err := proto.Unmarshal(bytes, rawReq); err != nil {
		return err
	}
	switch rawReq.GetType() {
	case proto_session.SessionRequestType_OPEN_SESSION_FOR_CONNECT:
		return validateSessionPayload(rawReq.GetData(), pool.AddSessionRequest.Get().(*proto_session.OpenSessionForConnectRequest), pool.AddSessionRequest.Put)
	case proto_session.SessionRequestType_TAKE_OVER_SESSION_OWNER:
		return validateSessionPayload(rawReq.GetData(), pool.TakeOverSessionOwnerRequest.Get().(*proto_session.TakeOverSessionOwnerRequest), pool.TakeOverSessionOwnerRequest.Put)
	case proto_session.SessionRequestType_REPLACE_SESSION_STATE_ON_CLEAN_START:
		return validateSessionPayload(rawReq.GetData(), pool.ReplaceSessionStateOnCleanStartRequest.Get().(*proto_session.ReplaceSessionStateOnCleanStartRequest), pool.ReplaceSessionStateOnCleanStartRequest.Put)
	case proto_session.SessionRequestType_DELETE_SESSION:
		return validateSessionPayload(rawReq.GetData(), pool.DeleteSessionRequest.Get().(*proto_session.DeleteSessionRequest), pool.DeleteSessionRequest.Put)
	case proto_session.SessionRequestType_DELETE_EXPIRED_SESSIONS:
		req := &proto_session.DeleteExpiredSessionsRequest{}
		return proto.Unmarshal(rawReq.GetData(), req)
	case proto_session.SessionRequestType_SAVE_OFFLINE_STATE:
		return validateSessionPayload(rawReq.GetData(), pool.UpdateSessionRequest.Get().(*proto_session.SaveOfflineStateRequest), pool.UpdateSessionRequest.Put)
	case proto_session.SessionRequestType_REMOVE_OUTGOING_UNFINISHED:
		return validateSessionPayload(rawReq.GetData(), pool.RemoveOutgoingUnfinishedRequest.Get().(*proto_session.RemoveOutgoingUnfinishedRequest), pool.RemoveOutgoingUnfinishedRequest.Put)
	case proto_session.SessionRequestType_UPSERT_INCOMING_UNFINISHED:
		return validateSessionPayload(rawReq.GetData(), pool.UpsertIncomingUnfinishedRequest.Get().(*proto_session.UpsertIncomingUnfinishedRequest), pool.UpsertIncomingUnfinishedRequest.Put)
	case proto_session.SessionRequestType_REMOVE_INCOMING_UNFINISHED:
		return validateSessionPayload(rawReq.GetData(), pool.RemoveIncomingUnfinishedRequest.Get().(*proto_session.RemoveIncomingUnfinishedRequest), pool.RemoveIncomingUnfinishedRequest.Put)
	case proto_session.SessionRequestType_COMMIT_OUTGOING_PROGRESS:
		return validateSessionPayload(rawReq.GetData(), &proto_session.CommitOutgoingProgressRequest{}, func(any) {})
	default:
		return fmt.Errorf("invalid request type of session state: %s", rawReq.GetType())
	}
}

type resettableProtoMessage interface {
	proto.Message
	Reset()
}

func validateSessionPayload[T resettableProtoMessage](data []byte, req T, put func(any)) error {
	defer func() {
		req.Reset()
		put(req)
	}()
	return proto.Unmarshal(data, req)
}

func (s *StateMachine) Update(bytes []byte) (dbsm.Result, error) {
	var (
		rawReq      = pool.RawRequest.Get().(*proto_session.SessionRequest)
		result      = dbsm.Result{}
		err         error
		ctx, cancel = context.WithTimeout(context.Background(), stateMachineRequestTimeout)
	)

	defer func() {
		rawReq.Reset()
		pool.RawRequest.Put(rawReq)
		cancel()
	}()

	if err := proto.Unmarshal(bytes, rawReq); err != nil {
		return result, err
	}

	switch rawReq.GetType() {
	case proto_session.SessionRequestType_OPEN_SESSION_FOR_CONNECT:
		req := pool.AddSessionRequest.Get().(*proto_session.OpenSessionForConnectRequest)
		defer func() {
			req.Reset()
			pool.AddSessionRequest.Put(req)
		}()
		if err := proto.Unmarshal(rawReq.GetData(), req); err != nil {
			return result, err
		}
		response, err := s.core.OpenSessionForConnect(ctx, req)
		if err != nil {
			return result, fmt.Errorf("open session for connect: %w", err)
		}
		logger.Logger.Debug().Bool("old_session_exists", response.GetOldSessionExists()).Msg("session center open session for connect")
		result.Data, err = proto.Marshal(response)
		if err != nil {
			return result, fmt.Errorf("marshal open session response: %w", err)
		}

	case proto_session.SessionRequestType_TAKE_OVER_SESSION_OWNER:
		req := pool.TakeOverSessionOwnerRequest.Get().(*proto_session.TakeOverSessionOwnerRequest)
		defer func() {
			req.Reset()
			pool.TakeOverSessionOwnerRequest.Put(req)
		}()
		if err := proto.Unmarshal(rawReq.GetData(), req); err != nil {
			return result, err
		}
		response, err := s.core.TakeOverSessionOwner(ctx, req)
		if err != nil {
			return result, fmt.Errorf("take over session owner: %w", err)
		}
		result.Data, err = proto.Marshal(response)
		if err != nil {
			return result, fmt.Errorf("marshal take over session owner response: %w", err)
		}

	case proto_session.SessionRequestType_REPLACE_SESSION_STATE_ON_CLEAN_START:
		req := pool.ReplaceSessionStateOnCleanStartRequest.Get().(*proto_session.ReplaceSessionStateOnCleanStartRequest)
		defer func() {
			req.Reset()
			pool.ReplaceSessionStateOnCleanStartRequest.Put(req)
		}()
		if err := proto.Unmarshal(rawReq.GetData(), req); err != nil {
			return result, err
		}
		err = s.core.ReplaceSessionStateOnCleanStart(ctx, req)

	case proto_session.SessionRequestType_DELETE_SESSION:
		req := pool.DeleteSessionRequest.Get().(*proto_session.DeleteSessionRequest)
		defer func() {
			req.Reset()
			pool.DeleteSessionRequest.Put(req)
		}()
		if err := proto.Unmarshal(rawReq.GetData(), req); err != nil {
			return result, err
		}
		err = s.core.DeleteSession(ctx, req)

	case proto_session.SessionRequestType_DELETE_EXPIRED_SESSIONS:
		req := &proto_session.DeleteExpiredSessionsRequest{}
		if err := proto.Unmarshal(rawReq.GetData(), req); err != nil {
			return result, err
		}
		response, e := s.core.DeleteExpiredSessions(ctx, req.GetNowUnixNano())
		err = e
		if err != nil {
			return result, fmt.Errorf("delete expired sessions: %w", err)
		}
		result.Data, err = proto.Marshal(&proto_session.DeleteExpiredSessionsResponse{
			ClientIDs: response,
		})
		if err != nil {
			return result, fmt.Errorf("marshal delete expired sessions response: %w", err)
		}

	case proto_session.SessionRequestType_SAVE_OFFLINE_STATE:
		req := pool.UpdateSessionRequest.Get().(*proto_session.SaveOfflineStateRequest)
		defer func() {
			req.Reset()
			pool.UpdateSessionRequest.Put(req)
		}()
		if err := proto.Unmarshal(rawReq.GetData(), req); err != nil {
			return result, err
		}
		err = s.core.SaveOfflineState(ctx, req)

	case proto_session.SessionRequestType_REMOVE_OUTGOING_UNFINISHED:
		req := pool.RemoveOutgoingUnfinishedRequest.Get().(*proto_session.RemoveOutgoingUnfinishedRequest)
		defer func() {
			req.Reset()
			pool.RemoveOutgoingUnfinishedRequest.Put(req)
		}()
		if err := proto.Unmarshal(rawReq.GetData(), req); err != nil {
			return result, err
		}
		err = s.core.RemoveOutgoingUnfinished(ctx, req)

	case proto_session.SessionRequestType_UPSERT_INCOMING_UNFINISHED:
		req := pool.UpsertIncomingUnfinishedRequest.Get().(*proto_session.UpsertIncomingUnfinishedRequest)
		defer func() {
			req.Reset()
			pool.UpsertIncomingUnfinishedRequest.Put(req)
		}()
		if err := proto.Unmarshal(rawReq.GetData(), req); err != nil {
			return result, err
		}
		err = s.core.UpsertIncomingUnfinished(ctx, req)

	case proto_session.SessionRequestType_REMOVE_INCOMING_UNFINISHED:
		req := pool.RemoveIncomingUnfinishedRequest.Get().(*proto_session.RemoveIncomingUnfinishedRequest)
		defer func() {
			req.Reset()
			pool.RemoveIncomingUnfinishedRequest.Put(req)
		}()
		if err := proto.Unmarshal(rawReq.GetData(), req); err != nil {
			return result, err
		}
		err = s.core.RemoveIncomingUnfinished(ctx, req)

	case proto_session.SessionRequestType_COMMIT_OUTGOING_PROGRESS:
		req := &proto_session.CommitOutgoingProgressRequest{}
		if err := proto.Unmarshal(rawReq.GetData(), req); err != nil {
			return result, err
		}
		err = s.core.CommitOutgoingProgress(ctx, req)

	default:
		return result, fmt.Errorf("invalid request type of session state: %s", rawReq.GetType())
	}

	if err != nil {
		return result, err
	}
	return result, nil
}

func (s *StateMachine) Lookup(i interface{}) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), stateMachineRequestTimeout)
	defer cancel()

	switch req := i.(type) {
	case *proto_session.ReadSessionRequest:
		return s.core.GetSession(ctx, req)
	case *proto_session.ReadSessionOwnerRequest:
		return s.core.GetSessionOwner(ctx, req)
	case *proto_session.ReadSessionOwnersRequest:
		return s.core.GetSessionOwners(ctx, req)
	default:
		logger.Logger.Error().Msg("invalid request type of session state")
		return nil, fmt.Errorf("invalid request type of session state")
	}
}

func (s *StateMachine) SaveSnapshot(writer io.Writer, collection dbsm.ISnapshotFileCollection, i <-chan struct{}) error {
	_ = collection
	_ = i
	return s.core.WriteSnapshot(writer)
}

func (s *StateMachine) RecoverFromSnapshot(reader io.Reader, files []dbsm.SnapshotFile, i <-chan struct{}) error {
	_ = files
	_ = i
	return s.core.RecoverSnapshot(reader)
}

func (s *StateMachine) Close() error {
	return nil
}
