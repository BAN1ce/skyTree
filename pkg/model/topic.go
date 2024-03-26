package model

import "github.com/BAN1ce/skyTree/pkg/broker/retain"

type Topic struct {
	Retain    *retain.Message `json:"retain"`
	SubClient []*Subscriber   `json:"sub_client"`
}

type Subscriber struct {
	ID   string `json:"id"`
	UUID string `json:"uuid"`
	QoS  int    `json:"qos"`
}
