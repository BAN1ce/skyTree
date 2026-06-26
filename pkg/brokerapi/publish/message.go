package publish

import (
	"sync/atomic"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/google/uuid"
)

type Message struct {
	SendOrReceive bool
	SendClientID  string
	OwnerToken    string
	MessageID     uuid.UUID
	PacketID      string
	AckReasonCode byte
	// ControlPacket is the complete MQTT control packet, used for serialization
	ControlPacket *packets.ControlPacket `json:"-"`
	// Publish is the Publish packet content, used for direct access without type assertion
	Publish *packets.Publish `json:"-"`

	// HasSendOnce is determined whether the message has been sent once
	HasSendOnce bool

	// PubReceived is determined whether the message has been received by the client
	// Sender -> Receiver: Publish
	// Receiver -> Sender: PubRec
	PubReceived bool
	// CreatedTime is the time when the message is created, unit is nanosecond
	CreatedTime int64
	ExpiredTime int64
	Will        bool
	WillDelay   time.Duration
	FromSession bool

	ShareTopic      string
	SubscribeTopic  string
	OwnerClientID   string
	Duplicate       bool
	Retain          bool
	RetainAsPublish bool
	RetryInfo       *RetryInfo
	EncodeData      []byte
}

func (m *Message) DeepCopy() *Message {
	return &Message{
		SendClientID:   m.SendClientID,
		OwnerToken:     m.OwnerToken,
		MessageID:      m.MessageID,
		PacketID:       m.PacketID,
		AckReasonCode:  m.AckReasonCode,
		ControlPacket:  m.ControlPacket,
		Publish:        m.Publish,
		PubReceived:    m.PubReceived,
		CreatedTime:    m.CreatedTime,
		ExpiredTime:    m.ExpiredTime,
		Will:           m.Will,
		ShareTopic:     m.ShareTopic,
		SubscribeTopic: m.SubscribeTopic,
		OwnerClientID:  m.OwnerClientID,
	}
}

func (m *Message) GetFullTopic() string {
	if m.Publish != nil {
		return m.Publish.Topic
	}
	return ""
}

func (m *Message) IsFromSession() bool {
	return m.FromSession
}

// SetFromSession marks whether this message is restored from session
func (m *Message) SetFromSession(fromSession bool) {
	m.FromSession = fromSession
}

// GetPublish returns the Publish packet, extracting it from ControlPacket if needed
func (m *Message) GetPublish() *packets.Publish {
	if m.Publish != nil {
		return m.Publish
	}
	if m.ControlPacket != nil {
		if publish, ok := m.ControlPacket.Content.(*packets.Publish); ok {
			m.Publish = publish
			return publish
		}
	}
	return nil
}

// GetControlPacket returns the ControlPacket, building it from Publish if needed
func (m *Message) GetControlPacket() *packets.ControlPacket {
	if m.ControlPacket != nil {
		return m.ControlPacket
	}
	if m.Publish != nil {
		cp := packets.NewControlPacket(packets.PUBLISH)
		cp.Content = m.Publish
		m.ControlPacket = cp
		return cp
	}
	return nil
}

// SetPublish sets the Publish packet and syncs ControlPacket
func (m *Message) SetPublish(publish *packets.Publish) {
	m.Publish = publish
	if publish != nil {
		if m.ControlPacket == nil {
			m.ControlPacket = packets.NewControlPacket(packets.PUBLISH)
		}
		// Use m.Publish to ensure Content always references the same object as m.Publish
		m.ControlPacket.Content = m.Publish
	} else {
		m.ControlPacket = nil
	}
}

// SetControlPacket sets the ControlPacket and extracts Publish
func (m *Message) SetControlPacket(cp *packets.ControlPacket) {
	m.ControlPacket = cp
	if cp != nil {
		if publish, ok := cp.Content.(*packets.Publish); ok {
			m.Publish = publish
		} else {
			m.Publish = nil
		}
	} else {
		m.Publish = nil
	}
}

func (m *Message) GetPublishRetain() bool {
	if m.Publish == nil {
		logger.Logger.Error().Msg("[publish] GetPublishRetain fail")
		return false
	}
	if m.Publish.ToControlPacket() == nil {
		logger.Logger.Error().Msg("[publish] GetPublishRetain ToControlPacket fail")
		return false
	}

	if p, ok := m.Publish.ToControlPacket().Content.(*packets.Publish); ok {
		return p.Retain
	} else {
		logger.Logger.Error().Msg("[publish] GetPublishRetain ToControlPacket Content.(*packets.Publish) fail")
		return false
	}
}

type RetryInfo struct {
	Key           string
	Times         atomic.Int32
	FirstPubTime  time.Time
	IntervalTime  time.Duration
	MaxRetryCount int
	Timeout       time.Duration
}

func (r *RetryInfo) IsTimeout() bool {
	maxRetryCount := r.MaxRetryCount
	if maxRetryCount <= 0 {
		maxRetryCount = 3
	}
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if int(r.Times.Load()) >= maxRetryCount {
		return true
	}

	if time.Since(r.FirstPubTime) > timeout {
		return true
	}
	return false
}
