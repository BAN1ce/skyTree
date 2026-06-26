package client

import (
	"github.com/BAN1ce/skyTree/config"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

const (
	maxUint16 = 1<<16 - 1
	maxUint32 = 1<<32 - 1
)

// applyConfigToConnAckProperties fills CONNACK properties from broker ConnectAckProperty config
// so that MQTT 5.0 capability negotiation is complete. Only non-empty optional fields are set.
func applyConfigToConnAckProperties(connAck *packets.ConnAck, prop *config.ConnectAckProperty) {
	if connAck == nil || prop == nil {
		return
	}
	if connAck.Properties == nil {
		connAck.Properties = &packets.ConnAckProperties{}
	}
	p := connAck.Properties

	// Receive Maximum (uint16, 1..65535)
	if prop.ReceiveMaximum > 0 {
		v := uint16(clampInt(prop.ReceiveMaximum, 1, maxUint16))
		p.ReceiveMaximum = &v
	}

	// Maximum QoS is only sent when the server restricts clients to QoS 0 or 1.
	// Supporting QoS 2 is represented by omitting the property.
	if prop.MaxQos >= 0 && prop.MaxQos <= 1 {
		v := byte(prop.MaxQos)
		p.MaximumQOS = &v
	}

	// Retain Available (byte, 0 or 1)
	if prop.RetainAvailable == 0 || prop.RetainAvailable == 1 {
		v := byte(prop.RetainAvailable)
		p.RetainAvailable = &v
	}

	// Maximum Packet Size (uint32)
	if prop.MaximumPacketSize > 0 {
		v := clampUint32(prop.MaximumPacketSize)
		p.MaximumPacketSize = &v
	}

	// Topic Alias Maximum (uint16)
	if prop.TopicAliasMaximum >= 0 {
		v := uint16(clampInt(prop.TopicAliasMaximum, 0, maxUint16))
		p.TopicAliasMaximum = &v
	}

	// Wildcard Subscription Available (byte 0/1)
	vWildcard := byte(0)
	if prop.WildcardSubscriptionAvailable {
		vWildcard = 1
	}
	p.WildcardSubAvailable = &vWildcard

	// Subscription Identifier Available (byte 0/1)
	vSubID := byte(0)
	if prop.SubscriptionIdentifierAvailable {
		vSubID = 1
	}
	p.SubIDAvailable = &vSubID

	// Shared Subscription Available (byte 0/1)
	vShared := byte(0)
	if prop.SharedSubscriptionAvailable {
		vShared = 1
	}
	p.SharedSubAvailable = &vShared

	// Server Keep Alive (uint16)
	if prop.ServerKeepAlive >= 0 {
		v := uint16(clampInt(prop.ServerKeepAlive, 0, maxUint16))
		p.ServerKeepAlive = &v
	}

	// Optional string properties: only set when non-empty
	if prop.ResponseInformation != "" {
		p.ResponseInfo = prop.ResponseInformation
	}
	if prop.ServerReference != "" && connAckUsesServerReference(connAck.ReasonCode) {
		p.ServerReference = prop.ServerReference
	}
}

func connAckUsesServerReference(reasonCode byte) bool {
	return reasonCode == packets.ConnAckUseAnotherServer || reasonCode == packets.ConnAckServerMoved
}

func applyConnectAwareConfigToConnAckProperties(connAck *packets.ConnAck, prop *config.ConnectAckProperty, connectPacket *packets.Connect) {
	applyConfigToConnAckProperties(connAck, prop)
	if connAck == nil || connAck.Properties == nil || prop == nil {
		return
	}
	if prop.ResponseInformation != "" && !connectRequestsResponseInfo(connectPacket) {
		connAck.Properties.ResponseInfo = ""
	}
}

func applyRuntimeCapabilitiesToConnAckProperties(connAck *packets.ConnAck, component *Component) {
	if connAck == nil {
		return
	}
	if connAck.Properties == nil {
		connAck.Properties = &packets.ConnAckProperties{}
	}
	if !runtimeSharedSubscriptionAvailable(component) {
		v := byte(0)
		connAck.Properties.SharedSubAvailable = &v
	}
}

func applyNegotiatedSessionExpiryToConnAckProperties(connAck *packets.ConnAck, sessionExpiryInterval uint32) {
	if connAck == nil {
		return
	}
	if connAck.Properties == nil {
		connAck.Properties = &packets.ConnAckProperties{}
	}
	connAck.Properties.SessionExpiryInterval = &sessionExpiryInterval
}

func applyEnhancedAuthToConnAckProperties(connAck *packets.ConnAck, method string, data []byte) {
	if connAck == nil || method == "" {
		return
	}
	if connAck.Properties == nil {
		connAck.Properties = &packets.ConnAckProperties{}
	}
	connAck.Properties.AuthMethod = method
	if len(data) > 0 {
		connAck.Properties.AuthData = cloneBytes(data)
	}
}

func runtimeSharedSubscriptionAvailable(component *Component) bool {
	return component != nil && component.sharedSubscriptionManager != nil
}

func connectRequestsResponseInfo(connectPacket *packets.Connect) bool {
	if connectPacket == nil || connectPacket.Properties == nil || connectPacket.Properties.RequestResponseInfo == nil {
		return false
	}
	return *connectPacket.Properties.RequestResponseInfo == 1
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampUint32(v int) uint32 {
	if v <= 0 {
		return 0
	}
	if v > maxUint32 {
		return maxUint32
	}
	return uint32(v)
}

// applyAssignedClientIDToConnAck sets MQTT5 CONNACK property "Assigned Client Identifier" when required.
func applyAssignedClientIDToConnAck(connAck *packets.ConnAck, assignedByServer bool, clientID string) {
	if connAck == nil {
		return
	}
	if !assignedByServer {
		return
	}
	if connAck.ReasonCode != packets.ConnAckSuccess {
		return
	}
	if connAck.Properties == nil {
		connAck.Properties = &packets.ConnAckProperties{}
	}
	connAck.Properties.AssignedClientID = clientID
}
