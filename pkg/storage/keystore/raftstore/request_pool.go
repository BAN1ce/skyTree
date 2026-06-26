package raftstore

import (
	"github.com/BAN1ce/skyTree/pkg/cluster/dbpb"
	"sync"
)

var (
	protoDBRequestPool = newRequestPool()
)

type requestPool struct {
	sync.Pool
}

func (p *requestPool) Get() *dbpb.Request {
	return p.Pool.Get().(*dbpb.Request)
}

func (p *requestPool) Put(b *dbpb.Request) {
	b.Reset()
	p.Pool.Put(b)
}

func newRequestPool() *requestPool {
	return &requestPool{
		sync.Pool{
			New: func() interface{} {
				return &dbpb.Request{}
			},
		},
	}
}
