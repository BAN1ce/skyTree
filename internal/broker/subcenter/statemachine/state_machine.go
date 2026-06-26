package statemachine

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/BAN1ce/skyTree/internal/broker/subcenter/memory"
	proto2 "github.com/BAN1ce/skyTree/proto/proto_topic"
	dbsm "github.com/lni/dragonboat/v3/statemachine"
	"google.golang.org/protobuf/proto"
)

// StateMachine 将订阅中心写请求应用到内存中心，并为 WAL/Raft 提供快照能力。
type StateMachine struct {
	center *memory.MemorySubCenter
}

const subCenterStateMachineTimeout = 3 * time.Second

var protoMarshal = proto.Marshal

// NewStateMachine 创建订阅中心状态机。
func NewStateMachine() *StateMachine {
	return &StateMachine{
		center: memory.NewMemorySubCenter(),
	}
}

func (t *StateMachine) ValidateUpdate(bytes []byte) error {
	req := new(proto2.UpdateRequest)
	if err := proto.Unmarshal(bytes, req); err != nil {
		return err
	}
	switch req.Type {
	case proto2.TopicRequestType_Sub:
		subReq := new(proto2.SubRequest)
		return proto.Unmarshal(req.Data, subReq)
	case proto2.TopicRequestType_UnSub:
		unSubReq := new(proto2.UnSubRequest)
		return proto.Unmarshal(req.Data, unSubReq)
	case proto2.TopicRequestType_DeleteClient:
		deleteClientReq := new(proto2.DeleteClientRequest)
		return proto.Unmarshal(req.Data, deleteClientReq)
	case proto2.TopicRequestType_DeleteTopic:
		deleteTopicReq := new(proto2.DeleteTopicRequest)
		return proto.Unmarshal(req.Data, deleteTopicReq)
	case proto2.TopicRequestType_SetClientOwnerToken:
		setReq := new(proto2.SetClientOwnerTokenRequest)
		return proto.Unmarshal(req.Data, setReq)
	default:
		return fmt.Errorf("unknown request type")
	}
}

// Update 处理订阅中心写请求，并把业务响应封装到 UpdateResponse 中返回。
func (t *StateMachine) Update(bytes []byte) (dbsm.Result, error) {
	var (
		req    = new(proto2.UpdateRequest)
		result = dbsm.Result{}
		err    error
		resp   = &proto2.UpdateResponse{
			RequestID: "",
			Type:      0,
		}
		ctx, cancel = context.WithTimeout(context.Background(), subCenterStateMachineTimeout)
	)
	defer cancel()

	if err := proto.Unmarshal(bytes, req); err != nil {
		return result, err
	}
	resp.RequestID = req.RequestID
	resp.Type = req.Type

	switch req.Type {
	case proto2.TopicRequestType_Sub:
		subReq := new(proto2.SubRequest)
		if err := proto.Unmarshal(req.Data, subReq); err != nil {
			return result, err
		}
		subResp, e := t.center.CreateSub(ctx, subReq)
		err = e
		if subResp != nil {
			b, marshalErr := protoMarshal(subResp)
			if marshalErr != nil {
				return result, fmt.Errorf("marshal sub response failed: %w", marshalErr)
			}
			resp.Data = b
		}

	case proto2.TopicRequestType_UnSub:
		unSubReq := new(proto2.UnSubRequest)
		if err := proto.Unmarshal(req.Data, unSubReq); err != nil {
			return result, err
		}
		unSubResp, e := t.center.DeleteSub(ctx, unSubReq)
		err = e
		if unSubResp != nil {
			b, marshalErr := protoMarshal(unSubResp)
			if marshalErr != nil {
				return result, fmt.Errorf("marshal unsub response failed: %w", marshalErr)
			}
			resp.Data = b
		}

	case proto2.TopicRequestType_DeleteClient:
		deleteClientReq := new(proto2.DeleteClientRequest)
		if err := proto.Unmarshal(req.Data, deleteClientReq); err != nil {
			return result, err
		}
		deleteClientResp, e := t.center.DeleteClient(ctx, deleteClientReq)
		err = e
		if deleteClientResp != nil {
			b, marshalErr := protoMarshal(deleteClientResp)
			if marshalErr != nil {
				return result, fmt.Errorf("marshal delete client response failed: %w", marshalErr)
			}
			resp.Data = b
		}

	case proto2.TopicRequestType_DeleteTopic:
		deleteTopicReq := new(proto2.DeleteTopicRequest)
		if err := proto.Unmarshal(req.Data, deleteTopicReq); err != nil {
			return result, err

		}
		deleteTopicResp, e := t.center.DeleteTopic(ctx, deleteTopicReq)
		err = e
		if deleteTopicResp != nil {
			b, marshalErr := protoMarshal(deleteTopicResp)
			if marshalErr != nil {
				return result, fmt.Errorf("marshal delete topic response failed: %w", marshalErr)
			}
			resp.Data = b
		}

	case proto2.TopicRequestType_SetClientOwnerToken:
		setReq := new(proto2.SetClientOwnerTokenRequest)
		if err := proto.Unmarshal(req.Data, setReq); err != nil {
			return result, err
		}
		setResp, e := t.center.SetClientOwnerToken(ctx, setReq)
		err = e
		if setResp != nil {
			b, marshalErr := protoMarshal(setResp)
			if marshalErr != nil {
				return result, fmt.Errorf("marshal set owner token response failed: %w", marshalErr)
			}
			resp.Data = b
		}

	default:
		return result, fmt.Errorf("unknown request type")
	}

	b, marshalErr := protoMarshal(resp)
	if marshalErr != nil {
		return result, fmt.Errorf("marshal update response failed: %w", marshalErr)
	}
	result.Data = b
	return result, err
}

// Lookup 处理订阅中心只读查询。
func (t *StateMachine) Lookup(i interface{}) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), subCenterStateMachineTimeout)
	defer cancel()

	switch req := i.(type) {
	case *proto2.GetAllMatchTopicsRequest:
		return t.center.GetAllMatchTopics(ctx, req)
	case *proto2.GetAllMatchTopicsForWildTopicRequest:
		return t.center.GetAllMatchTopicsForWildTopic(ctx, req)
	case *proto2.GetAllSubTopicClientRequest:
		return t.center.GetAllMatchClient(ctx, req)
	case *proto2.GetAllMatchClientV2Request:
		return t.center.GetAllMatchClientV2(ctx, req)
	case *proto2.GetSubTreeRequest:
		return t.center.GetSubTree(ctx, req)
	case *proto2.GetClientSubscriptionsRequest:
		return t.center.GetClientSubscriptions(ctx, req)
	case *proto2.GetShareGroupMembersRequest:
		return t.center.GetShareGroupMembers(ctx, req)
	default:
		return nil, fmt.Errorf("unknown request type")
	}
}

// SaveSnapshot 保存订阅中心快照。
func (t *StateMachine) SaveSnapshot(writer io.Writer, collection dbsm.ISnapshotFileCollection, i <-chan struct{}) error {
	_ = collection
	_ = i
	return t.center.WriteSnapshot(writer)
}

// RecoverFromSnapshot 从快照恢复订阅中心状态。
func (t *StateMachine) RecoverFromSnapshot(reader io.Reader, files []dbsm.SnapshotFile, i <-chan struct{}) error {
	_ = files
	_ = i
	return t.center.RecoverSnapshot(reader)
}

// Close 关闭状态机；当前没有额外资源需要释放。
func (t *StateMachine) Close() error {
	return nil
}
