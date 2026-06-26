package statemachine

import (
	"google.golang.org/protobuf/proto"

	"github.com/BAN1ce/skyTree/proto/proto_session"
)

func EncodeRequest(t proto_session.SessionRequestType, clientID string, requestNodeID uint64, msg proto.Message) ([]byte, error) {
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}
	req := &proto_session.SessionRequest{
		ClientID:      clientID,
		RequestNodeID: requestNodeID,
		Type:          t,
		Data:          data,
	}
	return proto.Marshal(req)
}
