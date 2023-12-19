package packet

import (
	"github.com/eclipse/paho.golang/packets"
)

const (
	PubReceived = 1 << iota
	FromSession
	Will
)

type Message struct {
	ClientID      string
	MessageID     string
	PacketID      string
	PublishPacket *packets.Publish `json:"-"`
	PubRelPacket  *packets.Pubrel  `json:"-"`
	PubReceived   bool
	Timestamp     int64
	ExpiredTime   int64
	Will          bool
	State         int
}

func (m *Message) SetPubReceived(received bool) {
	if received {
		m.State |= PubReceived
	} else {
		m.State &= ^PubReceived
	}
}

func (m *Message) SetFromSession(fromSession bool) {
	if fromSession {
		m.State |= FromSession
	} else {
		m.State &= ^FromSession
	}
}

func (m *Message) SetWill(will bool) {
	if will {
		m.State |= Will
	} else {
		m.State &= ^Will
	}
}

//

func (m *Message) IsPubReceived() bool {
	return m.State&PubReceived == PubReceived
}

func (m *Message) IsFromSession() bool {
	return m.State&FromSession == FromSession
}

func (m *Message) IsWill() bool {
	return m.State&Will == Will
}

func (m *Message) ToSessionPayload() string {

	return ""
}
