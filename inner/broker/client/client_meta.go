package client

import "time"

type Meta struct {
}

type Property struct {
	SessionExpiredInterval time.Duration
	ReceiveMaximum         uint16
	MaximumPacketSize      uint32
	TopicAliasMaximum      uint16
	RequestResponseInfo    bool
	RequestProblemInfo     bool
	UserProperty           map[string]string
	AuthenticationMethod   string
	AuthenticationData     []byte
}

type WillProperty struct {
	WillDelayInterval      time.Duration
	PayloadFormatIndicator bool
	MessageExpiryInterval  time.Duration
	ContentType            string
	ResponseTopic          string
	CorrelationData        []byte
	UserProperty           map[string]string
}
