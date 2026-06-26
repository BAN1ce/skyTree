package client

import (
	"testing"

	"github.com/BAN1ce/skyTree/config"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

func TestApplyAssignedClientIDToConnAck_SetsPropertyOnSuccessWhenAssigned(t *testing.T) {
	ca := &packets.ConnAck{
		ReasonCode:     packets.ConnAckSuccess,
		Properties:     nil,
		SessionPresent: false,
	}

	applyAssignedClientIDToConnAck(ca, true, "generated-id-123")

	if ca.Properties == nil {
		t.Fatalf("expected connack.Properties to be initialized")
	}
	if ca.Properties.AssignedClientID != "generated-id-123" {
		t.Fatalf("expected AssignedClientID to be %q, got %q", "generated-id-123", ca.Properties.AssignedClientID)
	}
}

func TestApplyAssignedClientIDToConnAck_DoesNothingWhenNotAssigned(t *testing.T) {
	ca := &packets.ConnAck{
		ReasonCode: packets.ConnAckSuccess,
	}

	applyAssignedClientIDToConnAck(ca, false, "should-not-set")

	if ca.Properties != nil && ca.Properties.AssignedClientID != "" {
		t.Fatalf("expected AssignedClientID to remain empty")
	}
}

func TestApplyAssignedClientIDToConnAck_DoesNothingOnNonSuccess(t *testing.T) {
	ca := &packets.ConnAck{
		ReasonCode: packets.ConnAckUnspecifiedError,
	}

	applyAssignedClientIDToConnAck(ca, true, "should-not-set")

	if ca.Properties != nil && ca.Properties.AssignedClientID != "" {
		t.Fatalf("expected AssignedClientID to remain empty")
	}
}

func TestApplyConfigToConnAckProperties_FillsAllCapabilityFields(t *testing.T) {
	prop := &config.ConnectAckProperty{
		ReceiveMaximum:                  100,
		MaxQos:                          2,
		RetainAvailable:                 1,
		MaximumPacketSize:               4096,
		TopicAliasMaximum:               10,
		WildcardSubscriptionAvailable:   true,
		SubscriptionIdentifierAvailable: true,
		SharedSubscriptionAvailable:     true,
		ServerKeepAlive:                 60,
		ResponseInformation:             "resp-info",
		ServerReference:                 "srv-ref",
	}
	ca := &packets.ConnAck{}

	applyConfigToConnAckProperties(ca, prop)

	if ca.Properties == nil {
		t.Fatal("expected Properties to be initialized")
	}
	p := ca.Properties

	if p.ReceiveMaximum == nil || *p.ReceiveMaximum != 100 {
		t.Errorf("ReceiveMaximum: want 100, got %v", p.ReceiveMaximum)
	}
	if p.MaximumQOS != nil {
		t.Errorf("MaximumQOS should be omitted when QoS 2 is supported, got %v", *p.MaximumQOS)
	}
	if p.RetainAvailable == nil || *p.RetainAvailable != 1 {
		t.Errorf("RetainAvailable: want 1, got %v", p.RetainAvailable)
	}
	if p.MaximumPacketSize == nil || *p.MaximumPacketSize != 4096 {
		t.Errorf("MaximumPacketSize: want 4096, got %v", p.MaximumPacketSize)
	}
	if p.TopicAliasMaximum == nil || *p.TopicAliasMaximum != 10 {
		t.Errorf("TopicAliasMaximum: want 10, got %v", p.TopicAliasMaximum)
	}
	if p.WildcardSubAvailable == nil || *p.WildcardSubAvailable != 1 {
		t.Errorf("WildcardSubAvailable: want 1, got %v", p.WildcardSubAvailable)
	}
	if p.SubIDAvailable == nil || *p.SubIDAvailable != 1 {
		t.Errorf("SubIDAvailable: want 1, got %v", p.SubIDAvailable)
	}
	if p.SharedSubAvailable == nil || *p.SharedSubAvailable != 1 {
		t.Errorf("SharedSubAvailable: want 1, got %v", p.SharedSubAvailable)
	}
	if p.ServerKeepAlive == nil || *p.ServerKeepAlive != 60 {
		t.Errorf("ServerKeepAlive: want 60, got %v", p.ServerKeepAlive)
	}
	if p.ResponseInfo != "resp-info" {
		t.Errorf("ResponseInfo: want %q, got %q", "resp-info", p.ResponseInfo)
	}
	if p.ServerReference != "" {
		t.Errorf("ServerReference should be omitted on successful CONNACK, got %q", p.ServerReference)
	}
	if p.AuthMethod != "" {
		t.Errorf("AuthMethod should not come from static config, got %q", p.AuthMethod)
	}
	if len(p.AuthData) != 0 {
		t.Errorf("AuthData should not come from static config, got %q", string(p.AuthData))
	}
}

func TestApplyConfigToConnAckProperties_ServerReferenceOnlyForRedirect(t *testing.T) {
	prop := &config.ConnectAckProperty{ServerReference: "mqtt://broker-b"}

	success := &packets.ConnAck{ReasonCode: packets.ConnAckSuccess}
	applyConfigToConnAckProperties(success, prop)
	if success.Properties != nil && success.Properties.ServerReference != "" {
		t.Fatalf("expected ServerReference omitted on success, got %q", success.Properties.ServerReference)
	}

	redirect := &packets.ConnAck{ReasonCode: packets.ConnAckUseAnotherServer}
	applyConfigToConnAckProperties(redirect, prop)
	if redirect.Properties == nil || redirect.Properties.ServerReference != "mqtt://broker-b" {
		t.Fatalf("expected redirect ServerReference, got %+v", redirect.Properties)
	}

	moved := &packets.ConnAck{ReasonCode: packets.ConnAckServerMoved}
	applyConfigToConnAckProperties(moved, prop)
	if moved.Properties == nil || moved.Properties.ServerReference != "mqtt://broker-b" {
		t.Fatalf("expected moved ServerReference, got %+v", moved.Properties)
	}
}

func TestApplyEnhancedAuthToConnAckPropertiesSetsNegotiatedAuth(t *testing.T) {
	ca := &packets.ConnAck{}

	applyEnhancedAuthToConnAckProperties(ca, "token", []byte("ok"))

	if ca.Properties == nil {
		t.Fatal("expected Properties to be initialized")
	}
	if ca.Properties.AuthMethod != "token" {
		t.Fatalf("expected negotiated AuthMethod token, got %q", ca.Properties.AuthMethod)
	}
	if string(ca.Properties.AuthData) != "ok" {
		t.Fatalf("expected negotiated AuthData ok, got %q", string(ca.Properties.AuthData))
	}
}

func TestApplyConnectAwareConfigToConnAckProperties_ResponseInfoOnlyWhenRequested(t *testing.T) {
	prop := &config.ConnectAckProperty{ResponseInformation: "resp-info"}

	notRequested := &packets.ConnAck{}
	applyConnectAwareConfigToConnAckProperties(notRequested, prop, &packets.Connect{Properties: &packets.ConnectProperties{}})
	if notRequested.Properties != nil && notRequested.Properties.ResponseInfo != "" {
		t.Fatalf("expected ResponseInfo to be omitted when client did not request it, got %q", notRequested.Properties.ResponseInfo)
	}

	request := byte(1)
	requested := &packets.ConnAck{}
	applyConnectAwareConfigToConnAckProperties(requested, prop, &packets.Connect{Properties: &packets.ConnectProperties{RequestResponseInfo: &request}})
	if requested.Properties == nil || requested.Properties.ResponseInfo != "resp-info" {
		t.Fatalf("expected ResponseInfo when requested, got %+v", requested.Properties)
	}
}

func TestApplyRuntimeCapabilitiesToConnAckProperties_DisablesSharedWhenManagerMissing(t *testing.T) {
	prop := &config.ConnectAckProperty{SharedSubscriptionAvailable: true}
	ca := &packets.ConnAck{}

	applyConnectAwareConfigToConnAckProperties(ca, prop, &packets.Connect{})
	applyRuntimeCapabilitiesToConnAckProperties(ca, &Component{})

	if ca.Properties == nil || ca.Properties.SharedSubAvailable == nil {
		t.Fatal("expected SharedSubAvailable to be set")
	}
	if *ca.Properties.SharedSubAvailable != 0 {
		t.Fatalf("expected shared subscriptions disabled without runtime manager, got %d", *ca.Properties.SharedSubAvailable)
	}
}

func TestApplyConfigToConnAckProperties_BoolToByteZeroWhenFalse(t *testing.T) {
	prop := &config.ConnectAckProperty{
		WildcardSubscriptionAvailable:   false,
		SubscriptionIdentifierAvailable: false,
		SharedSubscriptionAvailable:     false,
	}
	ca := &packets.ConnAck{}

	applyConfigToConnAckProperties(ca, prop)

	if ca.Properties.WildcardSubAvailable == nil || *ca.Properties.WildcardSubAvailable != 0 {
		t.Errorf("WildcardSubAvailable: want 0 when false, got %v", ca.Properties.WildcardSubAvailable)
	}
	if ca.Properties.SubIDAvailable == nil || *ca.Properties.SubIDAvailable != 0 {
		t.Errorf("SubIDAvailable: want 0 when false, got %v", ca.Properties.SubIDAvailable)
	}
	if ca.Properties.SharedSubAvailable == nil || *ca.Properties.SharedSubAvailable != 0 {
		t.Errorf("SharedSubAvailable: want 0 when false, got %v", ca.Properties.SharedSubAvailable)
	}
}

func TestApplyConfigToConnAckProperties_AdvertisesMaximumQOSOnlyWhenRestricted(t *testing.T) {
	tests := []struct {
		name   string
		maxQOS int
		want   byte
		set    bool
	}{
		{name: "restricts to qos0", maxQOS: 0, want: 0, set: true},
		{name: "restricts to qos1", maxQOS: 1, want: 1, set: true},
		{name: "omits qos2 default", maxQOS: 2, set: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ca := &packets.ConnAck{}
			applyConfigToConnAckProperties(ca, &config.ConnectAckProperty{MaxQos: tt.maxQOS})

			if tt.set {
				if ca.Properties.MaximumQOS == nil || *ca.Properties.MaximumQOS != tt.want {
					t.Fatalf("MaximumQOS: want %d, got %v", tt.want, ca.Properties.MaximumQOS)
				}
				return
			}
			if ca.Properties.MaximumQOS != nil {
				t.Fatalf("MaximumQOS should be omitted, got %d", *ca.Properties.MaximumQOS)
			}
		})
	}
}

func TestMQTT5SessionExpiryIntervalDefaultsToZeroWhenClientOmits(t *testing.T) {
	limits := config.BrokerLimits{SessionExpiryMaxSeconds: 30}

	got := mqtt5SessionExpiryInterval(&packets.Connect{Properties: &packets.ConnectProperties{}}, limits)

	if got != 0 {
		t.Fatalf("expected default session expiry 0, got %d", got)
	}
}

func TestMQTT5SessionExpiryIntervalUsesClientValueWhenProvided(t *testing.T) {
	clientExpiry := uint32(7)

	got := mqtt5SessionExpiryInterval(&packets.Connect{
		Properties: &packets.ConnectProperties{SessionExpiryInterval: &clientExpiry},
	}, config.BrokerLimits{})

	if got != 7 {
		t.Fatalf("expected client session expiry 7, got %d", got)
	}
}

func TestMQTT5SessionExpiryIntervalCapsClientValueByServerLimit(t *testing.T) {
	clientExpiry := uint32(120)

	got := mqtt5SessionExpiryInterval(&packets.Connect{
		Properties: &packets.ConnectProperties{SessionExpiryInterval: &clientExpiry},
	}, config.BrokerLimits{SessionExpiryMaxSeconds: 30})

	if got != 30 {
		t.Fatalf("expected capped session expiry 30, got %d", got)
	}
}

func TestApplyNegotiatedSessionExpiryToConnAckPropertiesUsesActualValue(t *testing.T) {
	ca := &packets.ConnAck{}
	applyConfigToConnAckProperties(ca, &config.ConnectAckProperty{})

	applyNegotiatedSessionExpiryToConnAckProperties(ca, 7)

	if ca.Properties == nil || ca.Properties.SessionExpiryInterval == nil {
		t.Fatal("expected SessionExpiryInterval to be set")
	}
	if got := *ca.Properties.SessionExpiryInterval; got != 7 {
		t.Fatalf("expected actual session expiry 7, got %d", got)
	}
}

func TestApplyConfigToConnAckProperties_EmptyStringsNotSet(t *testing.T) {
	prop := &config.ConnectAckProperty{
		ResponseInformation: "",
		ServerReference:     "",
		TopicAliasMaximum:   5, // set at least one so Properties is created
	}
	ca := &packets.ConnAck{}

	applyConfigToConnAckProperties(ca, prop)

	if ca.Properties.ResponseInfo != "" {
		t.Errorf("ResponseInfo should be empty when config empty, got %q", ca.Properties.ResponseInfo)
	}
	if ca.Properties.ServerReference != "" {
		t.Errorf("ServerReference should be empty when config empty, got %q", ca.Properties.ServerReference)
	}
	if ca.Properties.AuthMethod != "" {
		t.Errorf("AuthMethod should be empty when config empty, got %q", ca.Properties.AuthMethod)
	}
	if len(ca.Properties.AuthData) != 0 {
		t.Errorf("AuthData should be nil/empty when config empty, got %q", ca.Properties.AuthData)
	}
}

func TestApplyConfigToConnAckProperties_NilSafe(t *testing.T) {
	applyConfigToConnAckProperties(nil, &config.ConnectAckProperty{})
	applyConfigToConnAckProperties(&packets.ConnAck{}, nil)
	// no panic
}
