package client

import (
	"context"
	"crypto/x509"
	"testing"
)

func TestPeerCertificatesFromContextReturnsNilWhenAbsent(t *testing.T) {
	if certs := PeerCertificatesFromContext(context.Background()); certs != nil {
		t.Fatalf("expected nil when no peer cert in context, got %d", len(certs))
	}
	if certs := PeerCertificatesFromContext(nil); certs != nil {
		t.Fatalf("expected nil for nil context, got %d", len(certs))
	}
}

func TestPeerCertificatesFromContextRoundTrip(t *testing.T) {
	original := []*x509.Certificate{{Subject: x509.Certificate{}.Subject}}
	ctx := withPeerCertificates(context.Background(), original)
	got := PeerCertificatesFromContext(ctx)
	if len(got) != 1 {
		t.Fatalf("expected 1 cert in context, got %d", len(got))
	}
}

func TestWithPeerCertificatesNoOpForEmpty(t *testing.T) {
	parent := context.Background()
	ctx := withPeerCertificates(parent, nil)
	if ctx != parent {
		t.Fatal("expected withPeerCertificates(empty) to return parent unchanged")
	}
}
