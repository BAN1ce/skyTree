package pkg

type Queue interface {
	ReadByOffset(offset int64, limit int) []Message
	DeleteByOffset(offset int64)
}
