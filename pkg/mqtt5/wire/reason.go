package wire

import "github.com/BAN1ce/skyTree/pkg/mqtt5"

// validReasonCodes 按报文类型列出 MQTT 5 规范允许出现的原因码。
var validReasonCodes = map[mqtt5.PacketType]map[byte]struct{}{
	mqtt5.CONNACK: set(
		mqtt5.ConnAckSuccess,
		mqtt5.ConnAckUnspecifiedError,
		mqtt5.ConnAckMalformedPacket,
		mqtt5.ConnAckProtocolError,
		mqtt5.ConnAckImplementationSpecificError,
		mqtt5.ConnAckUnsupportedProtocolVersion,
		mqtt5.ConnAckInvalidClientID,
		mqtt5.ConnAckBadUsernameOrPassword,
		mqtt5.ConnAckNotAuthorized,
		mqtt5.ConnAckServerUnavailable,
		mqtt5.ConnAckServerBusy,
		mqtt5.ConnAckBanned,
		mqtt5.ConnAckBadAuthenticationMethod,
		mqtt5.ConnAckTopicNameInvalid,
		mqtt5.ConnAckPacketTooLarge,
		mqtt5.ConnAckQuotaExceeded,
		mqtt5.ConnAckPayloadFormatInvalid,
		mqtt5.ConnAckRetainNotSupported,
		mqtt5.ConnAckQoSNotSupported,
		mqtt5.ConnAckUseAnotherServer,
		mqtt5.ConnAckServerMoved,
		mqtt5.ConnAckConnectionRateExceeded,
	),
	mqtt5.PUBACK: set(
		mqtt5.PubackSuccess,
		mqtt5.PubackNoMatchingSubscribers,
		mqtt5.PubackUnspecifiedError,
		mqtt5.PubackImplementationSpecificError,
		mqtt5.PubackNotAuthorized,
		mqtt5.PubackTopicNameInvalid,
		mqtt5.PubackPacketIdentifierInUse,
		mqtt5.PubackQuotaExceeded,
		mqtt5.PubackPayloadFormatInvalid,
		mqtt5.PubackRetainNotSupported,
		mqtt5.PubackQoSNotSupported,
	),
	mqtt5.PUBREC: set(
		mqtt5.PubrecSuccess,
		mqtt5.PubrecNoMatchingSubscribers,
		mqtt5.PubrecUnspecifiedError,
		mqtt5.PubrecImplementationSpecificError,
		mqtt5.PubrecNotAuthorized,
		mqtt5.PubrecTopicNameInvalid,
		mqtt5.PubrecPacketIdentifierInUse,
		mqtt5.PubrecQuotaExceeded,
		mqtt5.PubrecPayloadFormatInvalid,
		mqtt5.PubrecRetainNotSupported,
		mqtt5.PubrecQoSNotSupported,
	),
	mqtt5.PUBREL: set(
		mqtt5.PubrelSuccess,
		mqtt5.PubrelPacketIdentifierNotFound,
	),
	mqtt5.PUBCOMP: set(
		mqtt5.PubcompSuccess,
		mqtt5.PubcompPacketIdentifierNotFound,
	),
	mqtt5.SUBACK: set(
		mqtt5.SubackGrantedQoS0,
		mqtt5.SubackGrantedQoS1,
		mqtt5.SubackGrantedQoS2,
		mqtt5.SubackUnspecifiederror,
		mqtt5.SubackImplementationspecificerror,
		mqtt5.SubackNotauthorized,
		mqtt5.SubackTopicFilterinvalid,
		mqtt5.SubackPacketIdentifierinuse,
		mqtt5.SubackQuotaexceeded,
		mqtt5.SubackSharedSubscriptionnotsupported,
		mqtt5.SubackSubscriptionIdentifiersnotsupported,
		mqtt5.SubackWildcardsubscriptionsnotsupported,
	),
	mqtt5.UNSUBACK: set(
		mqtt5.UnsubackSuccess,
		mqtt5.UnsubackNoSubscriptionFound,
		mqtt5.UnsubackUnspecifiedError,
		mqtt5.UnsubackImplementationSpecificError,
		mqtt5.UnsubackNotAuthorized,
		mqtt5.UnsubackTopicFilterInvalid,
		mqtt5.UnsubackPacketIdentifierInUse,
	),
	mqtt5.DISCONNECT: set(
		mqtt5.DisconnectNormalDisconnection,
		mqtt5.DisconnectDisconnectWithWillMessage,
		mqtt5.DisconnectUnspecifiedError,
		mqtt5.DisconnectMalformedPacket,
		mqtt5.DisconnectProtocolError,
		mqtt5.DisconnectImplementationSpecificError,
		mqtt5.DisconnectNotAuthorized,
		mqtt5.DisconnectServerBusy,
		mqtt5.DisconnectServerShuttingDown,
		mqtt5.DisconnectBadAuthenticationMethod,
		mqtt5.DisconnectKeepAliveTimeout,
		mqtt5.DisconnectSessionTakenOver,
		mqtt5.DisconnectTopicFilterInvalid,
		mqtt5.DisconnectTopicNameInvalid,
		mqtt5.DisconnectReceiveMaximumExceeded,
		mqtt5.DisconnectTopicAliasInvalid,
		mqtt5.DisconnectPacketTooLarge,
		mqtt5.DisconnectMessageRateTooHigh,
		mqtt5.DisconnectQuotaExceeded,
		mqtt5.DisconnectAdministrativeAction,
		mqtt5.DisconnectPayloadFormatInvalid,
		mqtt5.DisconnectRetainNotSupported,
		mqtt5.DisconnectQoSNotSupported,
		mqtt5.DisconnectUseAnotherServer,
		mqtt5.DisconnectServerMoved,
		mqtt5.DisconnectSharedSubscriptionNotSupported,
		mqtt5.DisconnectConnectionRateExceeded,
		mqtt5.DisconnectMaximumConnectTime,
		mqtt5.DisconnectSubscriptionIdentifiersNotSupported,
		mqtt5.DisconnectWildcardSubscriptionsNotSupported,
	),
	mqtt5.AUTH: set(
		mqtt5.AuthSuccess,
		mqtt5.AuthContinueAuthentication,
		mqtt5.AuthReauthenticate,
	),
}

// validateReason 校验指定原因码是否允许出现在对应报文中。
func validateReason(packet mqtt5.PacketType, code byte) error {
	if _, ok := validReasonCodes[packet][code]; ok {
		return nil
	}
	return malformed(packet, "reason code", "reason code is not valid for packet")
}

// set 把原因码列表转换成便于查表的集合。
func set(values ...byte) map[byte]struct{} {
	out := make(map[byte]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}
