package client

import "time"

type Property struct {
	SessionExpiryInterval time.Duration
	ReceiveMax            uint16
	PacketSizeMax         uint32
	TopicAliasMax         uint16
	RequestResponseInfo   string
	RequestProblemInfo    string
	UserProperty          map[string]string
}

func NewProperty() *Property {
	return &Property{
		UserProperty: make(map[string]string),
	}
}
