package statemachine

import (
	"fmt"

	proto "github.com/BAN1ce/skyTree/proto/proto_topic"
	dbsm "github.com/lni/dragonboat/v3/statemachine"
	proto2 "google.golang.org/protobuf/proto"
)

// EncodeUpdate 将订阅中心写请求编码成状态机可应用的 protobuf 字节。
func EncodeUpdate(msg proto2.Message) ([]byte, error) {
	cmd := &proto.UpdateRequest{}
	b, err := proto2.Marshal(msg)
	if err != nil {
		return nil, err
	}
	cmd.Data = b

	switch msg.(type) {
	case *proto.SubRequest:
		cmd.Type = proto.TopicRequestType_Sub
	case *proto.UnSubRequest:
		cmd.Type = proto.TopicRequestType_UnSub
	case *proto.DeleteClientRequest:
		cmd.Type = proto.TopicRequestType_DeleteClient
	case *proto.DeleteTopicRequest:
		cmd.Type = proto.TopicRequestType_DeleteTopic
	case *proto.SetClientOwnerTokenRequest:
		cmd.Type = proto.TopicRequestType_SetClientOwnerToken
	default:
		return nil, fmt.Errorf("unknown request type %T", msg)
	}

	return proto2.Marshal(cmd)
}

// DecodeUpdateResult 从状态机 apply 结果中解出 UpdateResponse，并把 Data 反序列化到 out。
func DecodeUpdateResult(res dbsm.Result, out proto2.Message) error {
	env := &proto.UpdateResponse{}
	if err := proto2.Unmarshal(res.Data, env); err != nil {
		return err
	}
	return proto2.Unmarshal(env.GetData(), out)
}
