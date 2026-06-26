package statemachine

import (
	"fmt"
	"io"
	"time"

	"github.com/BAN1ce/skyTree/internal/broker/willdelay/internal/state"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/proto/proto_will_delay"
	dbsm "github.com/lni/dragonboat/v3/statemachine"
	"google.golang.org/protobuf/proto"
)

// StateMachine applies will-delay updates for local WAL and Raft-backed stores.
type StateMachine struct {
	core *state.Core
}

// New creates a new StateMachine.
func New() *StateMachine {
	core := state.NewCore(1*time.Second, 3600)
	return &StateMachine{
		core: core,
	}
}

func (s *StateMachine) ValidateUpdate(bytes []byte) error {
	req := &proto_will_delay.WillDelayRequest{}
	if err := proto.Unmarshal(bytes, req); err != nil {
		return err
	}
	switch req.GetType() {
	case proto_will_delay.WillDelayRequestType_ADD_TASK:
		return state.ValidateTaskForAdd(req.GetTask())
	case proto_will_delay.WillDelayRequestType_DELETE_TASK:
		if req.ClientID == nil || *req.ClientID == "" {
			return fmt.Errorf("DELETE_TASK request missing clientID")
		}
		return nil
	default:
		return fmt.Errorf("invalid request type: %d", req.GetType())
	}
}

// Update handles write requests.
func (s *StateMachine) Update(bytes []byte) (dbsm.Result, error) {
	var (
		req    = &proto_will_delay.WillDelayRequest{}
		result = dbsm.Result{}
		err    error
	)

	if err = proto.Unmarshal(bytes, req); err != nil {
		logger.Logger.Error().Err(err).Msg("failed to unmarshal WillDelayRequest")
		return result, err
	}

	switch req.GetType() {
	case proto_will_delay.WillDelayRequestType_ADD_TASK:
		if err := state.ValidateTaskForAdd(req.GetTask()); err != nil {
			logger.Logger.Error().Err(err).Msg("invalid ADD_TASK request")
			return result, err
		}
		if err := s.core.AddTask(req.GetTask()); err != nil {
			return result, err
		}
		logger.Logger.Debug().
			Str("client_id", req.GetTask().GetClientID()).
			Int64("scheduled_time", req.GetTask().GetScheduledPublishTime()).
			Msg("added will delay task")

	case proto_will_delay.WillDelayRequestType_DELETE_TASK:
		if req.ClientID == nil || *req.ClientID == "" {
			logger.Logger.Error().Msg("DELETE_TASK request missing clientID")
			return result, fmt.Errorf("DELETE_TASK request missing clientID")
		}
		if req.Task != nil && req.Task.GetOwnerToken() != "" {
			s.core.DeleteTaskByOwner(*req.ClientID, req.Task.GetOwnerToken())
		} else {
			s.core.DeleteTask(*req.ClientID)
		}
		logger.Logger.Debug().
			Str("client_id", *req.ClientID).
			Msg("deleted will delay task")

	default:
		logger.Logger.Error().
			Int32("type", int32(req.GetType())).
			Msg("invalid request type")
		return result, fmt.Errorf("invalid request type: %d", req.GetType())
	}

	return result, nil
}

// Lookup handles read requests for due tasks.
func (s *StateMachine) Lookup(i interface{}) (interface{}, error) {
	req, ok := i.(*proto_will_delay.WillDelayRequest)
	if !ok {
		logger.Logger.Error().Msg("invalid request type for Lookup")
		return nil, fmt.Errorf("invalid request type %T", i)
	}

	if req.GetType() != proto_will_delay.WillDelayRequestType_GET_DUE_TASKS {
		return nil, fmt.Errorf("invalid request type for Lookup: %d", req.GetType())
	}

	if req.CurrentTime == nil {
		now := time.Now().UnixMicro()
		req.CurrentTime = &now
	}

	dueTasks := s.core.GetDueTasks(*req.CurrentTime)
	dueClientIDs := make([]string, 0, len(dueTasks))
	for _, task := range dueTasks {
		if task != nil {
			dueClientIDs = append(dueClientIDs, task.GetClientID())
		}
	}

	response := &proto_will_delay.WillDelayResponse{
		DueClientIDs: dueClientIDs,
		DueTasks:     dueTasks,
		Success:      true,
	}

	return response, nil
}

// SaveSnapshot saves the persistent task state.
func (s *StateMachine) SaveSnapshot(
	writer io.Writer,
	collection dbsm.ISnapshotFileCollection,
	done <-chan struct{},
) error {
	_, _ = collection, done

	snapshotState := s.core.GetState()
	data, err := proto.Marshal(snapshotState)
	if err != nil {
		logger.Logger.Error().Err(err).Msg("failed to marshal WillDelayState")
		return err
	}

	_, err = writer.Write(data)
	if err != nil {
		logger.Logger.Error().Err(err).Msg("failed to write snapshot")
		return err
	}

	logger.Logger.Info().
		Int("task_count", len(snapshotState.GetTasks())).
		Msg("saved will delay snapshot")

	return nil
}

// RecoverFromSnapshot recovers the persistent task state from a snapshot.
func (s *StateMachine) RecoverFromSnapshot(
	reader io.Reader,
	files []dbsm.SnapshotFile,
	done <-chan struct{},
) error {
	_, _ = files, done

	data, err := io.ReadAll(reader)
	if err != nil {
		logger.Logger.Error().Err(err).Msg("failed to read snapshot")
		return err
	}

	snapshotState := &proto_will_delay.WillDelayState{}
	if err = proto.Unmarshal(data, snapshotState); err != nil {
		logger.Logger.Error().Err(err).Msg("failed to unmarshal WillDelayState")
		return err
	}

	s.core.RecoverFromState(snapshotState)

	logger.Logger.Info().
		Int("task_count", len(snapshotState.GetTasks())).
		Msg("recovered will delay from snapshot")

	return nil
}

// Close closes the StateMachine.
func (s *StateMachine) Close() error {
	return nil
}
