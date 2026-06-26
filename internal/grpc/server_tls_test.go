package grpc

import (
	"testing"

	"github.com/BAN1ce/skyTree/config"
)

func TestServerOptionsRejectInsecureWhenNotAllowed(t *testing.T) {
	s := &Server{
		tlsConfig:     config.TLS{Enabled: false},
		allowInsecure: false,
	}
	options, err := s.serverOptions()
	if err == nil {
		t.Fatal("expected error when grpc tls disabled and allow_insecure=false")
	}
	if options != nil {
		t.Fatalf("expected nil options on error, got %+v", options)
	}
}

func TestServerOptionsAllowInsecureWhenExplicitlyEnabled(t *testing.T) {
	s := &Server{
		tlsConfig:     config.TLS{Enabled: false},
		allowInsecure: true,
	}
	options, err := s.serverOptions()
	if err != nil {
		t.Fatalf("expected allow_insecure grpc server options without error, got: %v", err)
	}
	if len(options) != 1 {
		t.Fatalf("expected metrics interceptor option for insecure mode, got len=%d", len(options))
	}
}
