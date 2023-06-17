package packet

const (
	DisconnectNormalDisconnection                 = 0x00
	DisconnectDisconnectWithWillMessage           = 0x04
	DisconnectUnspecifiedError                    = 0x80
	DisconnectMalformedPacket                     = 0x81
	DisconnectProtocolError                       = 0x82
	DisconnectImplementationSpecificError         = 0x83
	DisconnectNotAuthorized                       = 0x87
	DisconnectServerBusy                          = 0x89
	DisconnectServerShuttingDown                  = 0x8B
	DisconnectKeepAliveTimeout                    = 0x8D
	DisconnectSessionTakenOver                    = 0x8E
	DisconnectTopicFilterInvalid                  = 0x8F
	DisconnectTopicNameInvalid                    = 0x90
	DisconnectReceiveMaximumExceeded              = 0x93
	DisconnectTopicAliasInvalid                   = 0x94
	DisconnectPacketTooLarge                      = 0x95
	DisconnectMessageRateTooHigh                  = 0x96
	DisconnectQuotaExceeded                       = 0x97
	DisconnectAdministrativeAction                = 0x98
	DisconnectPayloadFormatInvalid                = 0x99
	DisconnectRetainNotSupported                  = 0x9A
	DisconnectQoSNotSupported                     = 0x9B
	DisconnectUseAnotherServer                    = 0x9C
	DisconnectServerMoved                         = 0x9D
	DisconnectSharedSubscriptionNotSupported      = 0x9E
	DisconnectConnectionRateExceeded              = 0x9F
	DisconnectMaximumConnectTime                  = 0xA0
	DisconnectSubscriptionIdentifiersNotSupported = 0xA1
	DisconnectWildcardSubscriptionsNotSupported   = 0xA2
	a
)

const (
	_ byte = iota
	CONNECT
	CONNACK
	PUBLISH
	PUBACK
	PUBREC
	PUBREL
	PUBCOMP
	SUBSCRIBE
	SUBACK
	UNSUBSCRIBE
	UNSUBACK
	PINGREQ
	PINGRESP
	DISCONNECT
	AUTH
)

const (
	PubackSuccess                     = 0x00
	PubackNoMatchingSubscribers       = 0x10
	PubackUnspecifiedError            = 0x80
	PubackImplementationSpecificError = 0x83
	PubackNotAuthorized               = 0x87
	PubackTopicNameInvalid            = 0x90
	PubackPacketIdentifierInUse       = 0x91
	PubackQuotaExceeded               = 0x97
	PubackPayloadFormatInvalid        = 0x99
)
