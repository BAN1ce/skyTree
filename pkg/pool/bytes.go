package pool

import "sync"

type BytePool struct {
	sync.Pool
}

func (p *BytePool) Get() []byte {
	return p.Pool.Get().([]byte)
}

func (p *BytePool) Put(b []byte) {
	p.Pool.Put(b)
}

func NewBytePool() *BytePool {
	return &BytePool{
		sync.Pool{
			New: func() interface{} {
				return make([]byte, 1024)
			},
		},
	}
}
