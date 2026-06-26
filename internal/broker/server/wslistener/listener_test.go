package wslistener

import (
	"net"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

func TestListenerAcceptsMQTTWebSocket(t *testing.T) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("resolve addr: %v", err)
	}
	l := NewListener(addr, nil)
	if err := l.Listen(); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()

	accepted := make(chan net.Conn, 1)
	acceptErr := make(chan error, 1)
	go func() {
		conn, err := l.Accept()
		if err != nil {
			acceptErr <- err
			return
		}
		accepted <- conn
	}()

	target := "ws://" + l.Name() + "/mqtt"
	origin := "http://" + l.Name() + "/"
	ws, err := websocket.Dial(target, "mqtt", origin)
	if err != nil {
		t.Fatalf("websocket dial: %v", err)
	}
	defer ws.Close()

	select {
	case conn := <-accepted:
		if conn == nil {
			t.Fatal("accepted nil connection")
		}
		_ = conn.Close()
	case err := <-acceptErr:
		t.Fatalf("accept: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for websocket connection")
	}
}

func TestListenerRejectsWebSocketWithoutMQTTSubprotocol(t *testing.T) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("resolve addr: %v", err)
	}
	l := NewListener(addr, nil)
	if err := l.Listen(); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()

	target := "ws://" + l.Name() + "/mqtt"
	origin := "http://" + l.Name() + "/"
	ws, err := websocket.Dial(target, "", origin)
	if err == nil {
		_ = ws.Close()
		t.Fatal("expected websocket handshake without mqtt subprotocol to fail")
	}
}
