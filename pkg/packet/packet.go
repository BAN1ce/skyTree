package packet

import (
	"github.com/eclipse/paho.golang/packets"
	"io"
)

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
	PublishAckSuccess                     = 0x00
	PublishAckNoMatchingSubscribers       = 0x10
	PublishAckUnspecifiedError            = 0x80
	PublishAckImplementationSpecificError = 0x83
	PublishAckNotAuthorized               = 0x87
	PublishAckTopicNameInvalid            = 0x90
	PublishAckPacketIdentifierInUse       = 0x91
	PublishAckQuotaExceeded               = 0x97
	PublishAckPayloadFormatInvalid        = 0x99
)

type StorePublishPacket interface {
	Encode() ([]byte, error)
	Decode([]byte) (packets.Publish, error)
}

func NewPublishAck() *packets.Puback {
	return packets.NewControlPacket(packets.PUBACK).Content.(*packets.Puback)
}
func NewPublishRec() *packets.Pubrec {
	return packets.NewControlPacket(packets.PUBREC).Content.(*packets.Pubrec)
}

func NewPublishComp() *packets.Pubcomp {
	return packets.NewControlPacket(packets.PUBCOMP).Content.(*packets.Pubcomp)
}

func WritePacket(writer io.Writer, packet packets.Packet) error {
	_, err := packet.WriteTo(writer)
	return err
}
