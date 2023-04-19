package meta

import (
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/proto"
	proto2 "github.com/golang/protobuf/proto"
)

type Command struct {
	commandType byte
	body        []byte
}

func DecodeCommand(data []byte) *Command {
	return &Command{
		commandType: data[len(data)-1],
		body:        data[:len(data)-1],
	}
}

func (c *Command) Encode() []byte {
	return append(c.body, c.commandType)
}

func (c *Command) ToSub() *proto.SubRequest {
	var (
		req = proto.SubRequest{}
	)
	if err := proto2.Unmarshal(c.body, &req); err != nil {
		logger.Logger.Error("unmarshal sub request error: ", err)
	}
	return &req
}
