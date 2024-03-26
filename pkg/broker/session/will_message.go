package session

import (
	"github.com/eclipse/paho.golang/packets"
	"time"
)

type WillProperties struct {
	*packets.Properties
}

func (w *WillProperties) GetDelayInterval() time.Duration {
	if w == nil || w.WillDelayInterval == nil {
		return 0
	}
	return time.Duration(int64(*w.WillDelayInterval)) * time.Second
}

func (w *WillProperties) GetFormat() byte {
	if w == nil || w.PayloadFormat == nil {
		return 0
	}
	return *w.PayloadFormat
}

func (w *WillProperties) GetMessageExpired() (time.Duration, bool) {
	if w == nil || w.MessageExpiry == nil {
		return 0, false
	}
	return time.Duration(int64(*w.MessageExpiry)) * time.Second, true
}

func (w *WillProperties) GetContentType() string {
	if w == nil {
		return ""
	}
	return w.ContentType
}

func (w *WillProperties) GetResponseTopic() string {
	if w == nil {
		return ""
	}
	return w.ResponseTopic
}

func (w *WillProperties) GetCorrelationData() []byte {
	if w == nil {
		return nil
	}
	return w.CorrelationData
}

func (w *WillProperties) GetUserProperties() []packets.User {
	if w == nil {
		return nil
	}
	return w.User
}

type WillMessage struct {
	Topic       string          `json:"topic"`
	QoS         int             `json:"qos"`
	Property    *WillProperties `json:"property"`
	Retain      bool            `json:"retain"`
	Payload     []byte          `json:"payload"`
	DelayTaskID string          `json:"delayTaskID"`
	CreatedTime string          `json:"createdTime"`
}

func (w *WillMessage) ToPublishPacket() *packets.Publish {
	return &packets.Publish{
		Topic:   w.Topic,
		QoS:     byte(w.QoS),
		Payload: w.Payload,
	}
}
