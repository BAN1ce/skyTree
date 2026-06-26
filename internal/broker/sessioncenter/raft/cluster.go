package raft

import (
	"context"
	"fmt"

	"github.com/BAN1ce/skyTree/internal/broker/sessioncenter/statemachine"
	"github.com/BAN1ce/skyTree/pkg/cluster"
	"github.com/BAN1ce/skyTree/proto/proto_session"
	"google.golang.org/protobuf/proto"
)

type Cluster struct {
	client      cluster.Client
	localNodeID uint64
}

func NewCluster(localNodeID uint64, client cluster.Client) *Cluster {
	return &Cluster{
		localNodeID: localNodeID,
		client:      client,
	}
}

func (c *Cluster) OpenSessionForConnect(ctx context.Context, request *proto_session.OpenSessionForConnectRequest) (*proto_session.OpenSessionForConnectResponse, error) {
	data, err := statemachine.EncodeRequest(
		proto_session.SessionRequestType_OPEN_SESSION_FOR_CONNECT,
		request.GetClientID(),
		c.client.GetNodeID(),
		request,
	)
	if err != nil {
		return nil, err
	}
	result, err := c.client.Write(ctx, data)
	if err != nil {
		return nil, err
	}

	response := proto_session.OpenSessionForConnectResponse{}
	err = proto.Unmarshal(result.Data, &response)
	return &response, err
}

func (c *Cluster) TakeOverSessionOwner(ctx context.Context, request *proto_session.TakeOverSessionOwnerRequest) (*proto_session.TakeOverSessionOwnerResponse, error) {
	clientID := ""
	if request != nil && request.GetOwner() != nil {
		clientID = request.GetOwner().GetClientID()
	}
	data, err := statemachine.EncodeRequest(
		proto_session.SessionRequestType_TAKE_OVER_SESSION_OWNER,
		clientID,
		c.client.GetNodeID(),
		request,
	)
	if err != nil {
		return nil, err
	}
	result, err := c.client.Write(ctx, data)
	if err != nil {
		return nil, err
	}

	response := proto_session.TakeOverSessionOwnerResponse{}
	err = proto.Unmarshal(result.Data, &response)
	return &response, err
}

func (c *Cluster) ReplaceSessionStateOnCleanStart(ctx context.Context, request *proto_session.ReplaceSessionStateOnCleanStartRequest) error {
	data, err := statemachine.EncodeRequest(
		proto_session.SessionRequestType_REPLACE_SESSION_STATE_ON_CLEAN_START,
		request.GetClientID(),
		uint64(c.localNodeID),
		request,
	)
	if err != nil {
		return err
	}
	_, err = c.client.Write(ctx, data)
	return err
}

func (c *Cluster) SaveOfflineState(ctx context.Context, request *proto_session.SaveOfflineStateRequest) error {
	data, err := statemachine.EncodeRequest(
		proto_session.SessionRequestType_SAVE_OFFLINE_STATE,
		request.GetClientID(),
		uint64(c.localNodeID),
		request,
	)
	if err != nil {
		return err
	}
	_, err = c.client.Write(ctx, data)
	return err
}

func (c *Cluster) RemoveOutgoingUnfinished(ctx context.Context, request *proto_session.RemoveOutgoingUnfinishedRequest) error {
	data, err := statemachine.EncodeRequest(
		proto_session.SessionRequestType_REMOVE_OUTGOING_UNFINISHED,
		request.GetClientID(),
		uint64(c.localNodeID),
		request,
	)
	if err != nil {
		return err
	}
	_, err = c.client.Write(ctx, data)
	return err
}

func (c *Cluster) UpsertIncomingUnfinished(ctx context.Context, request *proto_session.UpsertIncomingUnfinishedRequest) error {
	data, err := statemachine.EncodeRequest(
		proto_session.SessionRequestType_UPSERT_INCOMING_UNFINISHED,
		request.GetClientID(),
		uint64(c.localNodeID),
		request,
	)
	if err != nil {
		return err
	}
	_, err = c.client.Write(ctx, data)
	return err
}

func (c *Cluster) RemoveIncomingUnfinished(ctx context.Context, request *proto_session.RemoveIncomingUnfinishedRequest) error {
	data, err := statemachine.EncodeRequest(
		proto_session.SessionRequestType_REMOVE_INCOMING_UNFINISHED,
		request.GetClientID(),
		uint64(c.localNodeID),
		request,
	)
	if err != nil {
		return err
	}
	_, err = c.client.Write(ctx, data)
	return err
}

func (c *Cluster) CommitOutgoingProgress(ctx context.Context, request *proto_session.CommitOutgoingProgressRequest) error {
	data, err := statemachine.EncodeRequest(
		proto_session.SessionRequestType_COMMIT_OUTGOING_PROGRESS,
		request.GetClientID(),
		uint64(c.localNodeID),
		request,
	)
	if err != nil {
		return err
	}
	_, err = c.client.Write(ctx, data)
	return err
}

func (c *Cluster) DeleteSession(ctx context.Context, request *proto_session.DeleteSessionRequest) error {
	data, err := statemachine.EncodeRequest(
		proto_session.SessionRequestType_DELETE_SESSION,
		request.GetClientID(),
		uint64(c.localNodeID),
		request,
	)
	if err != nil {
		return err
	}
	_, err = c.client.Write(ctx, data)
	return err
}

func (c *Cluster) DeleteExpiredSessions(ctx context.Context, nowUnixNano int64) ([]string, error) {
	request := &proto_session.DeleteExpiredSessionsRequest{NowUnixNano: nowUnixNano}
	data, err := statemachine.EncodeRequest(
		proto_session.SessionRequestType_DELETE_EXPIRED_SESSIONS,
		"",
		uint64(c.localNodeID),
		request,
	)
	if err != nil {
		return nil, err
	}
	result, err := c.client.Write(ctx, data)
	if err != nil {
		return nil, err
	}
	response := &proto_session.DeleteExpiredSessionsResponse{}
	if err := proto.Unmarshal(result.Data, response); err != nil {
		return nil, err
	}
	return response.GetClientIDs(), nil
}

func (c *Cluster) GetSession(ctx context.Context, request *proto_session.ReadSessionRequest) (*proto_session.ReadSessionResponse, error) {
	result, err := c.client.Read(ctx, request)
	if err != nil {
		return nil, err
	}
	resp, ok := result.(*proto_session.ReadSessionResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T", result)
	}
	return resp, nil
}

func (c *Cluster) GetSessionOwner(ctx context.Context, request *proto_session.ReadSessionOwnerRequest) (*proto_session.ReadSessionOwnerResponse, error) {
	result, err := c.client.Read(ctx, request)
	if err != nil {
		return nil, err
	}
	resp, ok := result.(*proto_session.ReadSessionOwnerResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T", result)
	}
	return resp, nil
}

func (c *Cluster) GetSessionOwners(ctx context.Context, request *proto_session.ReadSessionOwnersRequest) (*proto_session.ReadSessionOwnersResponse, error) {
	result, err := c.client.Read(ctx, request)
	if err != nil {
		return nil, err
	}
	resp, ok := result.(*proto_session.ReadSessionOwnersResponse)
	if !ok {
		return nil, fmt.Errorf("unexpected response type %T", result)
	}
	return resp, nil
}
