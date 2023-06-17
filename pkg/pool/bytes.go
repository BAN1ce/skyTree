package pool

import (
	"bytes"
	"sync"
)

var (
	BytePool       = NewByte()
	ByteBufferPool = NewByteBufferPool()
)

type Byte struct {
	sync.Pool
}

func (p *Byte) Get() []byte {
	return p.Pool.Get().([]byte)
}

func (p *Byte) Put(b []byte) {
	p.Pool.Put(b)
}

func NewByte() *Byte {
	return &Byte{
		sync.Pool{
			New: func() interface{} {
				return make([]byte, 1024)
			},
		},
	}
}

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
