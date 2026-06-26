package server

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewServerReturnsErrorForUnsupportedProtocol(t *testing.T) {
	_, err := NewServer([]string{"mqtt://127.0.0.1:1883"})
	if err == nil {
		t.Fatal("expected error for unsupported protocol")
	}
	if !strings.Contains(err.Error(), "unsupported protocol") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewServerReturnsErrorWhenTLSListenerHasNoTLSConfig(t *testing.T) {
	_, err := NewServer([]string{"tls://127.0.0.1:8883"})
	if err == nil {
		t.Fatal("expected error for tls listener without tls config")
	}
	if !strings.Contains(err.Error(), "requires tls config") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewServerAllowsPlainTCPWhenMTLSModeOff(t *testing.T) {
	_, err := NewServer(
		[]string{"tcp://127.0.0.1:1883"},
		WithTLSFilesAndMTLS("", "", "", 0, "off"),
	)
	if err != nil {
		t.Fatalf("NewServer error: %v", err)
	}
}

func TestStartListenerExitsWhenContextCanceledDuringConnSend(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	listener := &singleConnListener{
		conn:     serverConn,
		accepted: make(chan struct{}),
		closed:   make(chan struct{}),
	}
	s := &Server{
		listener: []Listener{listener},
		conn:     make(chan net.Conn),
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.wg.Add(1)
	s.startListener(ctx)

	select {
	case <-listener.accepted:
	case <-time.After(time.Second):
		t.Fatal("listener did not accept the test connection")
	}

	cancel()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("listener goroutine did not exit after context cancellation")
	}
}

func TestCloseClosesAllListenersAndWaitsEvenWhenOneCloseFails(t *testing.T) {
	closeFailed := errors.New("close failed")
	l1 := newBlockingListener("listener-1", closeFailed)
	l2 := newBlockingListener("listener-2", nil)

	ctx, cancel := context.WithCancel(context.Background())
	s := &Server{
		listener: []Listener{l1, l2},
		conn:     make(chan net.Conn),
		started:  true,
		cancel:   cancel,
	}
	s.wg.Add(len(s.listener))
	s.startListener(ctx)

	if !l1.waitAccepted(time.Second) || !l2.waitAccepted(time.Second) {
		t.Fatal("listener goroutines did not start in time")
	}

	err := s.Close()
	if err == nil {
		t.Fatal("expected close error")
	}
	if !errors.Is(err, closeFailed) {
		t.Fatalf("expected aggregated close error to include close failure: %v", err)
	}
	if got := l1.closeCallCount(); got != 1 {
		t.Fatalf("listener-1 close calls = %d, want 1", got)
	}
	if got := l2.closeCallCount(); got != 1 {
		t.Fatalf("listener-2 close calls = %d, want 1", got)
	}
	if s.started {
		t.Fatal("server should be marked not started after Close")
	}
}

type singleConnListener struct {
	conn     net.Conn
	accepted chan struct{}
	closed   chan struct{}
	once     sync.Once
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	if l.conn != nil {
		conn := l.conn
		l.conn = nil
		close(l.accepted)
		return conn, nil
	}
	<-l.closed
	return nil, net.ErrClosed
}

func (l *singleConnListener) Close() error {
	l.once.Do(func() {
		close(l.closed)
	})
	return nil
}

func (l *singleConnListener) Listen() error { return nil }

func (l *singleConnListener) Name() string { return "single-conn" }

var _ Listener = (*singleConnListener)(nil)

type blockingListener struct {
	name      string
	closeErr  error
	accepted  chan struct{}
	closed    chan struct{}
	acceptMux sync.Once

	mu         sync.Mutex
	closeCalls int
	once       sync.Once
}

func newBlockingListener(name string, closeErr error) *blockingListener {
	return &blockingListener{
		name:     name,
		closeErr: closeErr,
		accepted: make(chan struct{}),
		closed:   make(chan struct{}),
	}
}

func (l *blockingListener) Accept() (net.Conn, error) {
	l.acceptMux.Do(func() {
		close(l.accepted)
	})
	<-l.closed
	return nil, net.ErrClosed
}

func (l *blockingListener) Close() error {
	l.mu.Lock()
	l.closeCalls++
	l.mu.Unlock()
	l.once.Do(func() {
		close(l.closed)
	})
	return l.closeErr
}

func (l *blockingListener) Listen() error {
	return nil
}

func (l *blockingListener) Name() string {
	return l.name
}

func (l *blockingListener) closeCallCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.closeCalls
}

func (l *blockingListener) waitAccepted(timeout time.Duration) bool {
	select {
	case <-l.accepted:
		return true
	case <-time.After(timeout):
		return false
	}
}

var _ Listener = (*blockingListener)(nil)
