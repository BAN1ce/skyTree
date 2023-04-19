package meta

import "github.com/BAN1ce/skyTree/pkg/proto"

func newClientModel(id string) *proto.ClientModel {
	var (
		p = new(proto.ClientModel)
	)
	p.Meta = map[string]string{}
	p.ID = id
	p.SubTopic = map[string]int32{}
	return p
}
