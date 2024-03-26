package model

import (
	"github.com/BAN1ce/skyTree/pkg/broker/session"
	"github.com/BAN1ce/skyTree/pkg/broker/topic"
)

type Session struct {
	ID              string                     `json:"id"`
	WillMessage     *session.WillMessage       `json:"will_message"`
	ConnectProperty *session.ConnectProperties `json:"connect_property"`
	ExpiryInterval  int64                      `json:"expiry_interval"`
	Topic           []topic.Meta               `json:"topic"`
}
