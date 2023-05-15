package pkg

import (
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/util"
)

type Packet struct {
	id      string
	topic   string
	payload []byte
}

func NewPacket(id, topic string, payload []byte) *Packet {
	return &Packet{
		id:      id,
		topic:   topic,
		payload: payload,
	}
}

func (p *Packet) Encode() []byte {
	return append(util.GenerateVariableData([]byte(p.id)), util.GenerateVariableData(p.payload)...)
}
func DecodePacket(rawData []byte) *Packet {
	var (
		p   = &Packet{}
		err error
		id  []byte
	)
	id, err = util.ParseVariableData(rawData)
	if err != nil {
		logger.Logger.Error("ParseVariableData id  error: ", err)
	}
	p.id = string(id)
	p.payload, err = util.ParseVariableData(rawData[len(id):])
	if err != nil {
		logger.Logger.Error("ParseVariableData payload error: ", err)
	}
	return p
}

func (p *Packet) GetID() string {
	return p.id
}

func (p *Packet) GetPayload() []byte {
	return p.payload
}

func (p *Packet) GetTopic() string {
	return p.topic
}

type Message interface {
	GetID() string
	GetPayload() []byte
	GetTopic() string
}
