package tcp

import (
	"net"
	"testing"
)

func TestListenerNameAfterListenFailure(t *testing.T) {
	t.Parallel()

	addr := &net.TCPAddr{
		IP:   net.ParseIP("203.0.113.1"),
		Port: 0,
	}
	l := NewListener(addr)

	if err := l.Listen(); err == nil {
		t.Fatal("expected Listen to fail for an unavailable local address")
	}

	if got := l.Name(); got != addr.String() {
		t.Fatalf("Name() = %q, want %q", got, addr.String())
	}
}
