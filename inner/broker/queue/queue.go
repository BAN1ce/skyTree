package queue

type Queue interface {
	Push(topic string, id string, data []byte) error
	ReadByID(topic string, id string) ([]byte, error)
	ReadFromOffset(topic string, offset int64, limit int64) ([][]byte, error)
	ReadFromID(topic string, id string, limit int64) ([][]byte, error)
	DeleteByID(topic string, id string) error
	DeleteByOffset(topic string, offset int64) error
}
