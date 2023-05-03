package pkg

type Message interface {
	GetOffset() int64
	GetBody() []byte
}
