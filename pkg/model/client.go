package model

import (
	"github.com/BAN1ce/skyTree/pkg/broker/topic"
	"time"
)

type Client struct {
	Session *Session `json:"session,omitempty"`
	ID      string   `json:"id"`
	UID     string   `json:"uid"`
	Meta    *Meta    `json:"meta,omitempty"`
}

type Meta struct {
	AliveTime time.Time    `json:"alive_time"`
	KeepAlive string       `json:"keep_alive"`
	Topic     []topic.Meta `json:"topic"`
}
