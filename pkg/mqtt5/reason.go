package mqtt5

const (
	// CONNACK 原因码：连接建立是否成功以及失败原因。
	ConnAckSuccess                     = 0x00
	ConnAckUnspecifiedError            = 0x80
	ConnAckMalformedPacket             = 0x81
	ConnAckProtocolError               = 0x82
	ConnAckImplementationSpecificError = 0x83
	ConnAckUnsupportedProtocolVersion  = 0x84
	ConnAckInvalidClientID             = 0x85
	ConnAckBadUsernameOrPassword       = 0x86
	ConnAckNotAuthorized               = 0x87
	ConnAckServerUnavailable           = 0x88
	ConnAckServerBusy                  = 0x89
	ConnAckBanned                      = 0x8A
	ConnAckBadAuthenticationMethod     = 0x8C
	ConnAckTopicNameInvalid            = 0x90
	ConnAckPacketTooLarge              = 0x95
	ConnAckQuotaExceeded               = 0x97
	ConnAckPayloadFormatInvalid        = 0x99
	ConnAckRetainNotSupported          = 0x9A
	ConnAckQoSNotSupported             = 0x9B
	ConnAckUseAnotherServer            = 0x9C
	ConnAckServerMoved                 = 0x9D
	ConnAckConnectionRateExceeded      = 0x9F

	// PUBACK 原因码：QoS 1 发布确认结果。
	PubackSuccess                     = 0x00
	PubackNoMatchingSubscribers       = 0x10
	PubackUnspecifiedError            = 0x80
	PubackImplementationSpecificError = 0x83
	PubackNotAuthorized               = 0x87
	PubackTopicNameInvalid            = 0x90
	PubackPacketIdentifierInUse       = 0x91
	PubackQuotaExceeded               = 0x97
	PubackPayloadFormatInvalid        = 0x99
	PubackRetainNotSupported          = 0x9A
	PubackQoSNotSupported             = 0x9B

	// PUBREC 原因码：QoS 2 发布接收阶段确认结果。
	PubrecSuccess                     = 0x00
	PubrecNoMatchingSubscribers       = 0x10
	PubrecUnspecifiedError            = 0x80
	PubrecImplementationSpecificError = 0x83
	PubrecNotAuthorized               = 0x87
	PubrecTopicNameInvalid            = 0x90
	PubrecPacketIdentifierInUse       = 0x91
	PubrecQuotaExceeded               = 0x97
	PubrecPayloadFormatInvalid        = 0x99
	PubrecRetainNotSupported          = 0x9A
	PubrecQoSNotSupported             = 0x9B

	// PUBREL 原因码：QoS 2 发布释放阶段确认结果。
	PubrelSuccess                  = 0x00
	PubrelPacketIdentifierNotFound = 0x92

	// PUBCOMP 原因码：QoS 2 发布完成阶段确认结果。
	PubcompSuccess                  = 0x00
	PubcompPacketIdentifierNotFound = 0x92

	// SUBACK 原因码：每个订阅主题过滤器的授权 QoS 或失败原因。
	SubackGrantedQoS0                         = 0x00
	SubackGrantedQoS1                         = 0x01
	SubackGrantedQoS2                         = 0x02
	SubackUnspecifiederror                    = 0x80
	SubackImplementationspecificerror         = 0x83
	SubackNotauthorized                       = 0x87
	SubackTopicFilterinvalid                  = 0x8F
	SubackPacketIdentifierinuse               = 0x91
	SubackQuotaexceeded                       = 0x97
	SubackSharedSubscriptionnotsupported      = 0x9E
	SubackSubscriptionIdentifiersnotsupported = 0xA1
	SubackWildcardsubscriptionsnotsupported   = 0xA2

	// UNSUBACK 原因码：每个取消订阅主题过滤器的处理结果。
	UnsubackSuccess                     = 0x00
	UnsubackNoSubscriptionFound         = 0x11
	UnsubackUnspecifiedError            = 0x80
	UnsubackImplementationSpecificError = 0x83
	UnsubackNotAuthorized               = 0x87
	UnsubackTopicFilterInvalid          = 0x8F
	UnsubackPacketIdentifierInUse       = 0x91

	// DISCONNECT 原因码：连接关闭的正常或异常原因。
	DisconnectNormalDisconnection                 = 0x00
	DisconnectDisconnectWithWillMessage           = 0x04
	DisconnectUnspecifiedError                    = 0x80
	DisconnectMalformedPacket                     = 0x81
	DisconnectProtocolError                       = 0x82
	DisconnectImplementationSpecificError         = 0x83
	DisconnectNotAuthorized                       = 0x87
	DisconnectServerBusy                          = 0x89
	DisconnectServerShuttingDown                  = 0x8B
	DisconnectBadAuthenticationMethod             = 0x8C
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

	// AUTH 原因码：扩展认证流程的继续、成功或重新认证状态。
	AuthSuccess                = 0x00
	AuthContinueAuthentication = 0x18
	AuthReauthenticate         = 0x19
)

// ReasonCode 表示 MQTT 5 原因码字节。
type ReasonCode = byte
