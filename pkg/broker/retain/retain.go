package retain

import "encoding/json"

type Message struct {
	Topic   string
	Payload []byte
}

func (m *Message) Encode() []byte {
	b, _ := json.Marshal(m)
	return b
}

func (m *Message) Decode(bytes []byte) error {
	return json.Unmarshal(bytes, &m)
}

type Retain interface {
	PutRetainMessage(message *Message) error
	GetRetainMessage(topic string) (*Message, bool)
	DeleteRetainMessage(topic string) error
}
