package bufferpool

import (
	"bytes"
	"sync"
)

var (
	ByteBufferPool = NewByteBufferPool()
)

type ByteBuffer struct {
	sync.Pool
}

func (p *ByteBuffer) Get() *bytes.Buffer {
	return p.Pool.Get().(*bytes.Buffer)
}

func (p *ByteBuffer) Put(b *bytes.Buffer) {
	b.Reset()
	p.Pool.Put(b)
}

func NewByteBufferPool() *ByteBuffer {
	return &ByteBuffer{
		sync.Pool{
			New: func() interface{} {
				return bytes.NewBuffer(make([]byte, 0, 512))
			},
		},
	}
}
