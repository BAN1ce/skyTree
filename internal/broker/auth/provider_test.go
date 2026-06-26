package auth

import (
	"testing"

	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
)

func TestValidateTimeoutBounds(t *testing.T) {
	if got := ValidateTimeout(0); got != 5 {
		t.Fatalf("expected default timeout 5, got %d", got)
	}
	if got := ValidateTimeout(-3); got != 5 {
		t.Fatalf("expected default timeout 5, got %d", got)
	}
	if got := ValidateTimeout(8); got != 8 {
		t.Fatalf("expected passthrough timeout 8, got %d", got)
	}
	if got := ValidateTimeout(99); got != 10 {
		t.Fatalf("expected capped timeout 10, got %d", got)
	}
}

func TestAuthRequestBodyBuildsProperties(t *testing.T) {
	req := authRequestBody("client-1", &packets.Auth{
		ReasonCode: packets.AuthContinueAuthentication,
		Properties: &packets.AuthProperties{
			AuthMethod:   "m",
			AuthData:     []byte("abc"),
			ReasonString: "ok",
			User: []packets.User{
				{Key: "k", Value: "v"},
			},
		},
	})

	if req["client_id"] != "client-1" {
		t.Fatalf("unexpected client_id: %v", req["client_id"])
	}
	if req["auth_method"] != "m" {
		t.Fatalf("unexpected auth_method: %v", req["auth_method"])
	}
	if _, ok := req["auth_data"]; !ok {
		t.Fatal("expected auth_data in request body")
	}
	if req["reason_string"] != "ok" {
		t.Fatalf("unexpected reason_string: %v", req["reason_string"])
	}
}

func TestParseAuthResponseBody(t *testing.T) {
	body := []byte(`{"reason_code":24,"properties":{"auth_method":"m","auth_data":"x","reason_string":"r","user_properties":[{"key":"k1","value":"v1"}]}}`)
	resp, err := parseAuthResponseBody(body)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if resp.ReasonCode != 24 {
		t.Fatalf("unexpected reason code: %d", resp.ReasonCode)
	}
	if resp.Properties == nil {
		t.Fatal("expected auth properties")
	}
	if resp.Properties.AuthMethod != "m" {
		t.Fatalf("unexpected auth method: %s", resp.Properties.AuthMethod)
	}
	if len(resp.Properties.User) != 1 || resp.Properties.User[0].Key != "k1" {
		t.Fatalf("unexpected user properties: %+v", resp.Properties.User)
	}
}
