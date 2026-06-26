package grpc

import (
	"context"
	"time"

	"github.com/BAN1ce/skyTree/internal/broker/client"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/grpc/nodepb"
	"github.com/BAN1ce/skyTree/pkg/metric"
)

const (
	ServiceCloseClientName = "ServiceCloseClient"
)

type ServiceCloseClient struct {
	nodepb.ClientCenterServer
	clientManager *client.Manager
}

func NewServiceCloseClient(manager *client.Manager) *ServiceCloseClient {
	return &ServiceCloseClient{
		clientManager: manager,
	}
}

func (s ServiceCloseClient) CloseClient(ctx context.Context, req *nodepb.CloseClientRequest) (*nodepb.CloseClientResponse, error) {
	startAt := time.Now()
	result := "error"
	defer func() {
		metric.RecordCloseClientServerStage("total", result, time.Since(startAt))
		metric.RecordRemoteClose(result, time.Since(startAt))
	}()

	var (
		response = &nodepb.CloseClientResponse{}
	)
	if s.clientManager == nil {
		metric.RecordRemoteCloseFailure("manager_missing")
		response.Message = "client manager missing"
		return response, nil
	}
	readClientStart := time.Now()
	c, ok := s.clientManager.ReadClient(req.GetClientID())
	readClientDuration := time.Since(readClientStart)

	logger.Logger.Info().
		Str("clientID", req.GetClientID()).
		Msg("close client from GRPC")
	if ok {
		// Fencing: only close if the token matches. Empty token means force-close (admin/legacy).
		if req.GetOwnerToken() != "" && c.GetOwnerToken() != req.GetOwnerToken() {
			result = "owner_conflict"
			metric.RecordOwnerTokenConflict("remote_close", "skip_close")
			metric.RecordRemoteCloseFailure("owner_conflict")
			response.Success = false
			response.Message = "owner token mismatch"
			metric.RecordCloseClientServerStage("read_client", result, readClientDuration)
			return response, nil
		}
		response.Success = true
		closeStart := time.Now()
		if err := c.CloseWithSessionTakenOver(); err != nil {
			result = "error"
			metric.RecordRemoteCloseFailure("close_error")
			logger.Logger.Error().Err(err).Msg("close client error")
		} else {
			result = "success"
		}
		metric.RecordCloseClientServerStage("read_client", result, readClientDuration)
		metric.RecordCloseClientServerStage("close_with_session_taken_over", result, time.Since(closeStart))
		deleteStart := time.Now()
		s.clientManager.DeleteClient(c)
		metric.RecordCloseClientServerStage("delete_client", result, time.Since(deleteStart))
	} else {
		result = "not_found"
		metric.RecordRemoteCloseFailure("not_found")
		response.Message = "client not found"
		metric.RecordCloseClientServerStage("read_client", result, readClientDuration)
	}
	return response, nil
}
