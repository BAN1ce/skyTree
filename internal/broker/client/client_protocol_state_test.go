package client

import (
	"bytes"
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/broker/plugin"
	"github.com/BAN1ce/skyTree/logger"
	clientcap "github.com/BAN1ce/skyTree/pkg/brokerapi/clientcap"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/wire"
)

func TestHandlePacketClosesWithoutDisconnectBeforeConnAck(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	c := NewClient(c1)
	c.ctx, c.cancel = context.WithCancelCause(context.Background())

	readCh := make(chan *packets.ControlPacket, 1)
	errCh := make(chan error, 1)
	go func() {
		_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
		cp, err := wire.Decode(c2, wire.DecodeOptions{})
		if err != nil {
			errCh <- err
			return
		}
		readCh <- cp
	}()

	err := NewClientHandler(c).HandlePacket(context.Background(), packets.NewControlPacket(packets.PINGREQ), c)
	if err != ErrProtocolError {
		t.Fatalf("expected protocol error, got %v", err)
	}

	select {
	case cp := <-readCh:
		t.Fatalf("server must close without DISCONNECT before successful CONNACK, got %s", cp.PacketType())
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected connection close")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for connection close")
	}
}

func TestRunSendsConnAckForUnsupportedProtocolVersion(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	c := NewClient(c1)
	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		c.Run(context.Background(), nil)
	}()

	clientID := "client-v4"
	body := []byte{
		0x00, 0x04, 'M', 'Q', 'T', 'T',
		0x04,
		0x02,
		0x00, 0x3c,
	}
	body = append(body, byte(len(clientID)>>8), byte(len(clientID)))
	body = append(body, []byte(clientID)...)
	raw := append([]byte{packets.CONNECT << 4, byte(len(body))}, body...)
	if _, err := c2.Write(raw); err != nil {
		t.Fatalf("write v4 CONNECT: %v", err)
	}

	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	cp, err := wire.Decode(c2, wire.DecodeOptions{})
	if err != nil {
		t.Fatalf("read CONNACK: %v", err)
	}
	connAck, ok := cp.Content.(*packets.ConnAck)
	if !ok {
		t.Fatalf("expected CONNACK, got %T", cp.Content)
	}
	if connAck.ReasonCode != packets.ConnAckUnsupportedProtocolVersion {
		t.Fatalf("expected Unsupported Protocol Version, got 0x%X", connAck.ReasonCode)
	}
	waitClientRunDone(t, runDone)
}

func TestRunClosesWithoutDisconnectForReadSizeViolationBeforeConnAck(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	cfg := mustLoadConfigForTest(t)
	oldMax := cfg.Broker.ConnectAckProperty.MaximumPacketSize
	cfg.Broker.ConnectAckProperty.MaximumPacketSize = 4
	defer func() {
		cfg.Broker.ConnectAckProperty.MaximumPacketSize = oldMax
	}()
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	c := NewClient(c1, WithConfig(clientConfigForTest(cfg.Broker)))
	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		c.Run(context.Background(), nil)
	}()

	if _, err := c2.Write([]byte{packets.PUBLISH << 4, 5}); err != nil {
		t.Fatalf("write oversize header: %v", err)
	}

	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	cp, err := wire.Decode(c2, wire.DecodeOptions{})
	if err == nil {
		t.Fatalf("server must close without DISCONNECT before successful CONNACK, got %s", cp.PacketType())
	}
	waitClientRunDone(t, runDone)
}

func waitClientRunDone(t *testing.T, done <-chan struct{}) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for client Run to exit")
	}
}

func TestWriteTrimsOptionalConnAckPropertiesToFitClientMaximum(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	clientMax := uint32(8)
	c := NewClient(c1)
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.clientMaximumPacketSize = &clientMax

	writeErrCh := make(chan error, 1)
	go func() {
		writeErrCh <- c.write(&clientcap.WritePacket{Packet: &packets.ControlPacket{
			FixedHeader: packets.FixedHeader{Type: packets.CONNACK},
			Content: &packets.ConnAck{
				ReasonCode: packets.ConnAckSuccess,
				Properties: &packets.ConnAckProperties{
					ReasonString: "this connack is intentionally too large",
					ResponseInfo: "response info that should be trimmed",
					User:         []packets.User{{Key: "debug", Value: "true"}},
				},
			},
		}})
	}()

	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	cp, readErr := wire.Decode(c2, wire.DecodeOptions{})
	if readErr != nil {
		t.Fatalf("read CONNACK: %v", readErr)
	}
	if err := <-writeErrCh; err != nil {
		t.Fatalf("expected CONNACK to be trimmed and written, got %v", err)
	}
	connAck, ok := cp.Content.(*packets.ConnAck)
	if !ok {
		t.Fatalf("expected CONNACK, got %T", cp.Content)
	}
	if connAck.Properties != nil &&
		(connAck.Properties.ReasonString != "" || connAck.Properties.ResponseInfo != "" || len(connAck.Properties.User) != 0) {
		t.Fatalf("expected optional CONNACK properties to be trimmed, got %+v", connAck.Properties)
	}
}

func TestTrimConnAckKeepsServerReferenceForRedirect(t *testing.T) {
	packet := &packets.ControlPacket{
		FixedHeader: packets.FixedHeader{Type: packets.CONNACK},
		Content: &packets.ConnAck{
			ReasonCode: packets.ConnAckUseAnotherServer,
			Properties: &packets.ConnAckProperties{
				ServerReference: "mqtt://broker-b.example.internal",
				ReasonString:    "this diagnostic text may be trimmed",
				ResponseInfo:    "response info may be trimmed",
				User:            []packets.User{{Key: "debug", Value: "true"}},
			},
		},
	}
	connAck := packet.Content.(*packets.ConnAck)

	NewClient(&callbackConn{}).trimConnAckPropertiesToFit(packet, connAck, 8)

	if connAck.Properties.ServerReference != "mqtt://broker-b.example.internal" {
		t.Fatalf("redirect CONNACK must preserve ServerReference, got %+v", connAck.Properties)
	}
}

func TestTrimDisconnectKeepsServerReferenceForRedirect(t *testing.T) {
	packet := &packets.ControlPacket{
		FixedHeader: packets.FixedHeader{Type: packets.DISCONNECT},
		Content: &packets.Disconnect{
			ReasonCode: packets.DisconnectServerMoved,
			Properties: &packets.DisconnectProperties{
				ServerReference: "mqtt://broker-c.example.internal",
				ReasonString:    "this diagnostic text may be trimmed",
				User:            []packets.User{{Key: "debug", Value: "true"}},
			},
		},
	}
	disconnect := packet.Content.(*packets.Disconnect)

	NewClient(&callbackConn{}).trimDisconnectPropertiesToFit(packet, disconnect, 8)

	if disconnect.Properties.ServerReference != "mqtt://broker-c.example.internal" {
		t.Fatalf("redirect DISCONNECT must preserve ServerReference, got %+v", disconnect.Properties)
	}
}

func TestWriteTrimsPubAckPropertiesToFitClientMaximum(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	clientMax := uint32(5)
	c := NewClient(c1)
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.clientMaximumPacketSize = &clientMax
	c.connAckAccepted.Store(true)

	writeErrCh := make(chan error, 1)
	go func() {
		writeErrCh <- c.write(&clientcap.WritePacket{Packet: &packets.ControlPacket{
			FixedHeader: packets.FixedHeader{Type: packets.PUBACK},
			Content: &packets.Puback{
				PacketID:   1,
				ReasonCode: packets.PubackNotAuthorized,
				Properties: &packets.PubackProperties{
					ReasonString: "this puback is intentionally too large",
					User:         []packets.User{{Key: "debug", Value: "true"}},
				},
			},
		}})
	}()

	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	cp, readErr := wire.Decode(c2, wire.DecodeOptions{})
	if readErr != nil {
		t.Fatalf("read PUBACK: %v", readErr)
	}
	if err := <-writeErrCh; err != nil {
		t.Fatalf("expected PUBACK to be trimmed and written, got %v", err)
	}
	puback, ok := cp.Content.(*packets.Puback)
	if !ok {
		t.Fatalf("expected PUBACK, got %T", cp.Content)
	}
	if puback.Properties != nil && (puback.Properties.ReasonString != "" || len(puback.Properties.User) != 0) {
		t.Fatalf("expected optional PUBACK properties to be trimmed, got %+v", puback.Properties)
	}
}

func TestWriteTrimsSubAckPropertiesToFitClientMaximum(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	clientMax := uint32(6)
	c := NewClient(c1)
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.clientMaximumPacketSize = &clientMax
	c.connAckAccepted.Store(true)

	writeErrCh := make(chan error, 1)
	go func() {
		writeErrCh <- c.write(&clientcap.WritePacket{Packet: &packets.ControlPacket{
			FixedHeader: packets.FixedHeader{Type: packets.SUBACK},
			Content: &packets.Suback{
				PacketID: 1,
				Reasons:  []byte{packets.SubackGrantedQoS0},
				Properties: &packets.SubackProperties{
					ReasonString: "this suback is intentionally too large",
					User:         []packets.User{{Key: "debug", Value: "true"}},
				},
			},
		}})
	}()

	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	cp, readErr := wire.Decode(c2, wire.DecodeOptions{})
	if readErr != nil {
		t.Fatalf("read SUBACK: %v", readErr)
	}
	if err := <-writeErrCh; err != nil {
		t.Fatalf("expected SUBACK to be trimmed and written, got %v", err)
	}
	suback, ok := cp.Content.(*packets.Suback)
	if !ok {
		t.Fatalf("expected SUBACK, got %T", cp.Content)
	}
	if suback.Properties != nil && (suback.Properties.ReasonString != "" || len(suback.Properties.User) != 0) {
		t.Fatalf("expected optional SUBACK properties to be trimmed, got %+v", suback.Properties)
	}
}

// MQTT5 §3.1.2.11.4：超过客户端 Maximum Packet Size 的下行 PUBLISH 必须被丢弃，
// 不能因此断连。
func TestWriteDiscardsOversizedOutboundPublish(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	clientMax := uint32(3)
	conn := &callbackConn{}
	c := NewClient(conn)
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.clientMaximumPacketSize = &clientMax
	c.connAckAccepted.Store(true)

	err := c.write(&clientcap.WritePacket{Packet: &packets.ControlPacket{
		FixedHeader: packets.FixedHeader{Type: packets.PUBLISH},
		Content: &packets.Publish{
			Topic:      "oversized/topic",
			Payload:    []byte("payload"),
			Properties: &packets.PublishProperties{},
		},
	}})
	if !IsOversizedOutboundPublish(err) {
		t.Fatalf("expected oversized publish error, got %v", err)
	}
	if len(conn.Bytes()) != 0 {
		t.Fatalf("expected no bytes written for discarded oversized publish, got %d bytes", len(conn.Bytes()))
	}
}

func TestWriteDoesNotApplyServerConfiguredMaximumAsOutboundFallback(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	cfg := mustLoadConfigForTest(t)
	oldMax := cfg.Broker.ConnectAckProperty.MaximumPacketSize
	cfg.Broker.ConnectAckProperty.MaximumPacketSize = 4
	defer func() {
		cfg.Broker.ConnectAckProperty.MaximumPacketSize = oldMax
	}()

	conn := &callbackConn{}
	c := NewClient(conn, WithConfig(clientConfigForTest(cfg.Broker)))
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.connAckAccepted.Store(true)
	// clientMaximumPacketSize intentionally left nil: client did not send CONNECT Maximum Packet Size.

	err := c.write(&clientcap.WritePacket{Packet: &packets.ControlPacket{
		FixedHeader: packets.FixedHeader{Type: packets.PUBLISH},
		Content: &packets.Publish{
			Topic:      "oversized/topic",
			Payload:    []byte("payload"),
			Properties: &packets.PublishProperties{},
		},
	}})
	if err != nil {
		t.Fatalf("expected outbound publish to ignore server maximum fallback, got %v", err)
	}
	if len(conn.Bytes()) == 0 {
		t.Fatal("expected publish bytes to be written")
	}
}

// 对非 PUBLISH 报文（例如 SUBACK），超长时仍然走 DISCONNECT 路径；
// 当完整 DISCONNECT 也超过客户端最大值时，应当回退为 minimal 版本。
func TestWriteSendsMinimalPacketTooLargeDisconnectWithinClientMaximum(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	clientMax := uint32(4)
	conn := &callbackConn{}
	c := NewClient(conn)
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.clientMaximumPacketSize = &clientMax
	c.connAckAccepted.Store(true)

	subackCP := packets.NewControlPacket(packets.SUBACK)
	subackCP.Content = &packets.Suback{
		PacketID: 1,
		Reasons:  []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	}
	err := c.write(&clientcap.WritePacket{Packet: subackCP})
	if err == nil {
		t.Fatal("expected packet size error for oversized non-publish")
	}

	cp, readErr := wire.Decode(bytes.NewReader(conn.Bytes()), wire.DecodeOptions{MaxPacketSize: int(clientMax)})
	if readErr != nil {
		t.Fatalf("expected minimal DISCONNECT within client maximum, got read error: %v", readErr)
	}
	disc, ok := cp.Content.(*packets.Disconnect)
	if !ok {
		t.Fatalf("expected DISCONNECT, got %T", cp.Content)
	}
	if disc.ReasonCode != packets.DisconnectPacketTooLarge {
		t.Fatalf("expected Packet Too Large, got 0x%X", disc.ReasonCode)
	}
	if disc.Properties != nil && disc.Properties.ReasonString != "" {
		t.Fatalf("expected reason string to be omitted to fit client maximum, got %q", disc.Properties.ReasonString)
	}
}

func TestHandleConnectDoesNotPublishWillWhenConnAckWriteFails(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	willCh := make(chan *brokerpublish.Message, 1)
	conn := &callbackConn{}
	c := NewClient(
		conn,
		WithSessionCenter(&fakeSessionCenter{}),
		WithSubCenter(&recordingSubCenter{}),
		WithClientManager(NewManager()),
		WithNotifyWillMessageChan(willCh),
	)
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	defer c.cancel(nil)

	clientMax := uint32(8)
	err := NewClientHandler(c).handleConnect(&packets.Connect{
		ProtocolName:    "MQTT",
		ProtocolVersion: 5,
		ClientID:        "client-will-before-connack",
		CleanStart:      true,
		WillFlag:        true,
		WillTopic:       "will/topic",
		WillMessage:     []byte("bye"),
		Properties: &packets.ConnectProperties{
			MaximumPacketSize: &clientMax,
		},
	})
	if err == nil {
		t.Fatal("expected CONNACK write to fail because it exceeds client maximum packet size")
	}

	select {
	case msg := <-willCh:
		t.Fatalf("will must not be published before successful CONNACK, got %+v", msg.GetPublish())
	default:
	}
}

func TestInitialEnhancedAuthRespectsConnectMaximumPacketSize(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	plugins := &plugin.Plugins{}
	plugins.OnReceivedAuth = []plugin.OnReceivedAuth{
		func(ctx context.Context, clientID string, auth *packets.Auth) (*packets.Auth, error) {
			return &packets.Auth{
				ReasonCode: packets.AuthContinueAuthentication,
				Properties: &packets.AuthProperties{
					AuthMethod: "token",
					AuthData:   []byte("this challenge is intentionally too large for the client's maximum packet size"),
				},
			}, nil
		},
	}

	c := NewClient(c1, WithPlugin(plugins))
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	defer c.cancel(nil)

	clientMax := uint32(8)
	errCh := make(chan error, 1)
	go func() {
		errCh <- NewClientHandler(c).handleConnect(&packets.Connect{
			ProtocolName:    "MQTT",
			ProtocolVersion: 5,
			ClientID:        "client-auth-max",
			CleanStart:      true,
			Properties: &packets.ConnectProperties{
				AuthMethod:        "token",
				MaximumPacketSize: &clientMax,
			},
		})
	}()

	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	cp, readErr := wire.Decode(c2, wire.DecodeOptions{})
	if readErr == nil {
		t.Fatalf("server must close without AUTH when initial AUTH exceeds client maximum, got %s", cp.PacketType())
	}
	if err := <-errCh; err == nil {
		t.Fatal("expected handleConnect to report packet size error")
	}
}

func TestInitialEnhancedAuthSuppressesProblemInfoWhenConnectRequestsNone(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	plugins := &plugin.Plugins{}
	plugins.OnReceivedAuth = []plugin.OnReceivedAuth{
		func(ctx context.Context, clientID string, auth *packets.Auth) (*packets.Auth, error) {
			return &packets.Auth{
				ReasonCode: packets.AuthContinueAuthentication,
				Properties: &packets.AuthProperties{
					AuthMethod:   "token",
					AuthData:     []byte("challenge"),
					ReasonString: "extra diagnostic",
					User:         []packets.User{{Key: "debug", Value: "true"}},
				},
			}, nil
		},
	}

	c := NewClient(c1, WithPlugin(plugins))
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	defer c.cancel(nil)

	noProblemInfo := byte(0)
	errCh := make(chan error, 1)
	go func() {
		errCh <- NewClientHandler(c).handleConnect(&packets.Connect{
			ProtocolName:    "MQTT",
			ProtocolVersion: 5,
			ClientID:        "client-auth-problem-info",
			CleanStart:      true,
			Properties: &packets.ConnectProperties{
				AuthMethod:         "token",
				RequestProblemInfo: &noProblemInfo,
			},
		})
	}()

	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	cp, err := wire.Decode(c2, wire.DecodeOptions{})
	if err != nil {
		t.Fatalf("read AUTH: %v", err)
	}
	auth, ok := cp.Content.(*packets.Auth)
	if !ok {
		t.Fatalf("expected AUTH, got %T", cp.Content)
	}
	if auth.Properties != nil && (auth.Properties.ReasonString != "" || len(auth.Properties.User) != 0) {
		t.Fatalf("expected problem information stripped, got %+v", auth.Properties)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("handleConnect: %v", err)
	}
}

func TestHandleAuthRejectsUnsolicitedAuthPacket(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	c := NewClient(c1)
	c.ID = "client-a"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.connAckAccepted.Store(true)

	readCh := make(chan *packets.ControlPacket, 1)
	errCh := make(chan error, 1)
	go func() {
		_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
		cp, err := wire.Decode(c2, wire.DecodeOptions{})
		if err != nil {
			errCh <- err
			return
		}
		readCh <- cp
	}()

	err := NewClientHandler(c).handleAuth(context.Background(), &packets.Auth{
		ReasonCode: packets.AuthReauthenticate,
		Properties: &packets.AuthProperties{
			AuthMethod: "token",
		},
	})
	if err != ErrProtocolError {
		t.Fatalf("expected protocol error, got %v", err)
	}

	select {
	case cp := <-readCh:
		disc, ok := cp.Content.(*packets.Disconnect)
		if !ok {
			t.Fatalf("expected DISCONNECT, got %T", cp.Content)
		}
		if disc.ReasonCode != packets.DisconnectProtocolError {
			t.Fatalf("expected Protocol Error, got 0x%X", disc.ReasonCode)
		}
	case err := <-errCh:
		t.Fatalf("read packet error: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for DISCONNECT")
	}
}

func TestHandlePacketRejectsSecondConnect(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	c := NewClient(c1)
	c.ID = "client-a"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.connAckAccepted.Store(true)

	readCh := make(chan *packets.ControlPacket, 1)
	errCh := make(chan error, 1)
	go func() {
		_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
		cp, err := wire.Decode(c2, wire.DecodeOptions{})
		if err != nil {
			errCh <- err
			return
		}
		readCh <- cp
	}()

	err := NewClientHandler(c).HandlePacket(context.Background(), packets.NewControlPacket(packets.CONNECT), c)
	if err != ErrProtocolError {
		t.Fatalf("expected protocol error, got %v", err)
	}

	select {
	case cp := <-readCh:
		disc, ok := cp.Content.(*packets.Disconnect)
		if !ok {
			t.Fatalf("expected DISCONNECT, got %T", cp.Content)
		}
		if disc.ReasonCode != packets.DisconnectProtocolError {
			t.Fatalf("expected Protocol Error, got 0x%X", disc.ReasonCode)
		}
	case err := <-errCh:
		t.Fatalf("read packet error: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for DISCONNECT")
	}
}

func TestHandlePacketRejectsServerOnlyPacketAfterConnAck(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	c := NewClient(c1)
	c.ID = "client-server-only"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.connAckAccepted.Store(true)

	readCh := make(chan *packets.ControlPacket, 1)
	errCh := make(chan error, 1)
	go func() {
		_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
		cp, err := wire.Decode(c2, wire.DecodeOptions{})
		if err != nil {
			errCh <- err
			return
		}
		readCh <- cp
	}()

	err := NewClientHandler(c).HandlePacket(context.Background(), packets.NewControlPacket(packets.PINGRESP), c)
	if err != ErrProtocolError {
		t.Fatalf("expected protocol error, got %v", err)
	}

	select {
	case cp := <-readCh:
		disc, ok := cp.Content.(*packets.Disconnect)
		if !ok {
			t.Fatalf("expected DISCONNECT, got %T", cp.Content)
		}
		if disc.ReasonCode != packets.DisconnectProtocolError {
			t.Fatalf("expected Protocol Error, got 0x%X", disc.ReasonCode)
		}
	case err := <-errCh:
		t.Fatalf("read packet error: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for DISCONNECT")
	}
}

func TestHandleDisconnectRejectsSessionExpiryAfterZeroConnectExpiry(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	c := NewClient(c1)
	c.ID = "client-a"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.sessionExpiryInterval = 0
	c.connectSessionExpiryInterval = 0
	c.connAckAccepted.Store(true)

	readCh := make(chan *packets.ControlPacket, 1)
	errCh := make(chan error, 1)
	go func() {
		_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
		cp, err := wire.Decode(c2, wire.DecodeOptions{})
		if err != nil {
			errCh <- err
			return
		}
		readCh <- cp
	}()

	expiry := uint32(30)
	err := NewClientHandler(c).handleDisconnect(context.Background(), &packets.Disconnect{
		ReasonCode: 0,
		Properties: &packets.DisconnectProperties{
			SessionExpiryInterval: &expiry,
		},
	})
	if err != ErrProtocolError {
		t.Fatalf("expected protocol error, got %v", err)
	}

	select {
	case cp := <-readCh:
		disc, ok := cp.Content.(*packets.Disconnect)
		if !ok {
			t.Fatalf("expected DISCONNECT, got %T", cp.Content)
		}
		if disc.ReasonCode != packets.DisconnectProtocolError {
			t.Fatalf("expected Protocol Error, got 0x%X", disc.ReasonCode)
		}
	case err := <-errCh:
		t.Fatalf("read packet error: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for DISCONNECT")
	}
}

func TestHandleDisconnectCapsSessionExpiryAboveBrokerLimit(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	cfg := mustLoadConfigForTest(t)
	oldMax := cfg.Broker.Limits.SessionExpiryMaxSeconds
	cfg.Broker.Limits.SessionExpiryMaxSeconds = 10
	defer func() {
		cfg.Broker.Limits.SessionExpiryMaxSeconds = oldMax
	}()
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	c := NewClient(c1, WithConfig(clientConfigForTest(cfg.Broker)))
	c.ID = "client-a"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.sessionExpiryInterval = 5
	c.connectSessionExpiryInterval = 5
	c.connAckAccepted.Store(true)

	expiry := uint32(11)
	err := NewClientHandler(c).handleDisconnect(context.Background(), &packets.Disconnect{
		ReasonCode: 0,
		Properties: &packets.DisconnectProperties{
			SessionExpiryInterval: &expiry,
		},
	})
	if err != nil {
		t.Fatalf("expected disconnect to be accepted with capped expiry, got %v", err)
	}
	if c.sessionExpiryInterval != 10 {
		t.Fatalf("expected DISCONNECT Session Expiry to be capped to 10, got %d", c.sessionExpiryInterval)
	}

	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	if cp, readErr := wire.Decode(c2, wire.DecodeOptions{}); readErr == nil {
		t.Fatalf("normal DISCONNECT handling should close without server DISCONNECT, got %s", cp.PacketType())
	}
}

func TestHandleDisconnectCallsReceivedDisconnectPlugin(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	called := false
	c := NewClient(&callbackConn{}, WithPlugin(&plugin.Plugins{
		PacketPlugin: plugin.PacketPlugin{
			OnReceivedDisconnect: []plugin.OnReceivedDisconnect{
				func(_ context.Context, clientID string, disconnect *packets.Disconnect) error {
					called = true
					if clientID != "disconnect-plugin-test" {
						t.Fatalf("expected client id disconnect-plugin-test, got %q", clientID)
					}
					if disconnect.ReasonCode != packets.DisconnectNormalDisconnection {
						t.Fatalf("expected normal disconnection, got 0x%X", disconnect.ReasonCode)
					}
					return nil
				},
			},
		},
	}))
	c.ID = "disconnect-plugin-test"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())

	err := NewClientHandler(c).handleDisconnect(context.Background(), &packets.Disconnect{
		ReasonCode: packets.DisconnectNormalDisconnection,
	})
	if err != nil {
		t.Fatalf("handle disconnect: %v", err)
	}
	if !called {
		t.Fatal("expected disconnect plugin to be called")
	}
}

func TestHandleDisconnectAcceptsZeroSessionExpiryAfterZeroConnectExpiry(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	c := NewClient(c1)
	c.ID = "client-a"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.sessionExpiryInterval = 30
	c.connectSessionExpiryInterval = 0
	c.connAckAccepted.Store(true)

	expiry := uint32(0)
	err := NewClientHandler(c).handleDisconnect(context.Background(), &packets.Disconnect{
		ReasonCode: 0,
		Properties: &packets.DisconnectProperties{
			SessionExpiryInterval: &expiry,
		},
	})
	if err != nil {
		t.Fatalf("expected zero DISCONNECT Session Expiry to be accepted, got %v", err)
	}
	if c.sessionExpiryInterval != 0 || !c.cleanSession {
		t.Fatalf("expected session expiry to be updated to zero, got expiry=%d clean=%v", c.sessionExpiryInterval, c.cleanSession)
	}

	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	if cp, readErr := wire.Decode(c2, wire.DecodeOptions{}); readErr == nil {
		t.Fatalf("normal DISCONNECT handling should close without server DISCONNECT, got %s", cp.PacketType())
	}
}

func TestHandleDisconnectRejectsServerReferenceFromClient(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	c := NewClient(c1)
	c.ID = "client-a"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.sessionExpiryInterval = 30
	c.connectSessionExpiryInterval = 30
	c.connAckAccepted.Store(true)

	readCh := make(chan *packets.ControlPacket, 1)
	errCh := make(chan error, 1)
	go func() {
		_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
		cp, err := wire.Decode(c2, wire.DecodeOptions{})
		if err != nil {
			errCh <- err
			return
		}
		readCh <- cp
	}()

	err := NewClientHandler(c).handleDisconnect(context.Background(), &packets.Disconnect{
		ReasonCode: packets.DisconnectNormalDisconnection,
		Properties: &packets.DisconnectProperties{
			ServerReference: "mqtt://redirect-broker",
		},
	})
	if err != ErrProtocolError {
		t.Fatalf("expected protocol error, got %v", err)
	}

	select {
	case cp := <-readCh:
		disc, ok := cp.Content.(*packets.Disconnect)
		if !ok {
			t.Fatalf("expected DISCONNECT, got %T", cp.Content)
		}
		if disc.ReasonCode != packets.DisconnectProtocolError {
			t.Fatalf("expected Protocol Error, got 0x%X", disc.ReasonCode)
		}
	case err := <-errCh:
		t.Fatalf("read packet error: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for DISCONNECT")
	}
}

func TestHandleDisconnectRejectsServerOnlyReasonCodeFromClient(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	c := NewClient(c1)
	c.ID = "client-a"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.sessionExpiryInterval = 30
	c.connectSessionExpiryInterval = 30
	c.connAckAccepted.Store(true)

	readCh := make(chan *packets.ControlPacket, 1)
	errCh := make(chan error, 1)
	go func() {
		_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
		cp, err := wire.Decode(c2, wire.DecodeOptions{})
		if err != nil {
			errCh <- err
			return
		}
		readCh <- cp
	}()

	err := NewClientHandler(c).handleDisconnect(context.Background(), &packets.Disconnect{
		ReasonCode: packets.DisconnectServerBusy,
	})
	if err != ErrProtocolError {
		t.Fatalf("expected protocol error, got %v", err)
	}

	select {
	case cp := <-readCh:
		disc, ok := cp.Content.(*packets.Disconnect)
		if !ok {
			t.Fatalf("expected DISCONNECT, got %T", cp.Content)
		}
		if disc.ReasonCode != packets.DisconnectProtocolError {
			t.Fatalf("expected Protocol Error, got 0x%X", disc.ReasonCode)
		}
	case err := <-errCh:
		t.Fatalf("read packet error: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for DISCONNECT")
	}
}

func TestHandleDisconnectAcceptsAdministrativeActionFromClient(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	c := NewClient(c1)
	c.ID = "client-a"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.sessionExpiryInterval = 30
	c.connectSessionExpiryInterval = 30
	c.connAckAccepted.Store(true)

	err := NewClientHandler(c).handleDisconnect(context.Background(), &packets.Disconnect{
		ReasonCode: packets.DisconnectAdministrativeAction,
	})
	if err != nil {
		t.Fatalf("expected administrative action disconnect to be accepted, got %v", err)
	}

	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	if cp, readErr := wire.Decode(c2, wire.DecodeOptions{}); readErr == nil {
		t.Fatalf("accepted client DISCONNECT should close without server DISCONNECT, got %s", cp.PacketType())
	}
}

func TestHandleConnectFailureRespectsMaximumPacketSizeBeforeValidation(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	c := NewClient(c1)
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	defer c.cancel(nil)

	clientMax := uint32(8)
	errCh := make(chan error, 1)
	go func() {
		errCh <- NewClientHandler(c).handleConnect(&packets.Connect{
			ProtocolName:    "MQTT",
			ProtocolVersion: 5,
			ClientID:        "client-max-precheck",
			CleanStart:      true,
			WillFlag:        true,
			WillTopic:       "bad/#",
			WillMessage:     []byte("bye"),
			Properties: &packets.ConnectProperties{
				MaximumPacketSize: &clientMax,
			},
		})
	}()

	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	cp, err := wire.Decode(c2, wire.DecodeOptions{MaxPacketSize: int(clientMax)})
	if err != nil {
		t.Fatalf("expected failure CONNACK within client maximum packet size, got %v", err)
	}
	connAck, ok := cp.Content.(*packets.ConnAck)
	if !ok {
		t.Fatalf("expected CONNACK, got %T", cp.Content)
	}
	if connAck.ReasonCode != packets.ConnAckTopicNameInvalid {
		t.Fatalf("expected Topic Name Invalid, got 0x%X", connAck.ReasonCode)
	}
	if connAck.Properties != nil && connAck.Properties.ReasonString != "" {
		t.Fatalf("expected reason string to be trimmed under client maximum packet size, got %q", connAck.Properties.ReasonString)
	}

	if err := <-errCh; err != ErrProtocolError {
		t.Fatalf("expected protocol error from invalid CONNECT, got %v", err)
	}
}

func TestHandleConnectRejectsWillRetainWhenRetainUnavailable(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	cfg := mustLoadConfigForTest(t)
	oldRetainAvailable := cfg.Broker.ConnectAckProperty.RetainAvailable
	cfg.Broker.ConnectAckProperty.RetainAvailable = 0
	defer func() {
		cfg.Broker.ConnectAckProperty.RetainAvailable = oldRetainAvailable
	}()
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	c := NewClient(c1, WithConfig(clientConfigForTest(cfg.Broker)))
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	defer c.cancel(nil)

	errCh := make(chan error, 1)
	go func() {
		errCh <- NewClientHandler(c).handleConnect(&packets.Connect{
			ProtocolName:    "MQTT",
			ProtocolVersion: 5,
			ClientID:        "client-will-retain",
			CleanStart:      true,
			WillFlag:        true,
			WillRetain:      true,
			WillTopic:       "will/topic",
			WillMessage:     []byte("bye"),
			Properties:      &packets.ConnectProperties{},
		})
	}()

	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	cp, err := wire.Decode(c2, wire.DecodeOptions{})
	if err != nil {
		t.Fatalf("read connack: %v", err)
	}
	connAck, ok := cp.Content.(*packets.ConnAck)
	if !ok {
		t.Fatalf("expected CONNACK, got %T", cp.Content)
	}
	if connAck.ReasonCode != packets.ConnAckRetainNotSupported {
		t.Fatalf("expected Retain Not Supported, got 0x%X", connAck.ReasonCode)
	}
	if err := <-errCh; !errors.Is(err, ErrProtocolError) {
		t.Fatalf("expected protocol error, got %v", err)
	}
}

func TestHandleConnectRejectsWillQoSAboveMaximumQoS(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	cfg := mustLoadConfigForTest(t)
	oldMaxQoS := cfg.Broker.ConnectAckProperty.MaxQos
	cfg.Broker.ConnectAckProperty.MaxQos = 1
	defer func() {
		cfg.Broker.ConnectAckProperty.MaxQos = oldMaxQoS
	}()
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	c := NewClient(c1, WithConfig(clientConfigForTest(cfg.Broker)))
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	defer c.cancel(nil)

	errCh := make(chan error, 1)
	go func() {
		errCh <- NewClientHandler(c).handleConnect(&packets.Connect{
			ProtocolName:    "MQTT",
			ProtocolVersion: 5,
			ClientID:        "client-will-qos",
			CleanStart:      true,
			WillFlag:        true,
			WillQOS:         2,
			WillTopic:       "will/topic",
			WillMessage:     []byte("bye"),
			Properties:      &packets.ConnectProperties{},
		})
	}()

	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	cp, err := wire.Decode(c2, wire.DecodeOptions{})
	if err != nil {
		t.Fatalf("read connack: %v", err)
	}
	connAck, ok := cp.Content.(*packets.ConnAck)
	if !ok {
		t.Fatalf("expected CONNACK, got %T", cp.Content)
	}
	if connAck.ReasonCode != packets.ConnAckQoSNotSupported {
		t.Fatalf("expected QoS Not Supported, got 0x%X", connAck.ReasonCode)
	}
	if err := <-errCh; !errors.Is(err, ErrProtocolError) {
		t.Fatalf("expected protocol error, got %v", err)
	}
}

func TestEnhancedAuthCompletesConnectAfterAuthSuccess(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	var calls int
	plugins := &plugin.Plugins{}
	plugins.OnReceivedAuth = []plugin.OnReceivedAuth{
		func(ctx context.Context, clientID string, auth *packets.Auth) (*packets.Auth, error) {
			calls++
			if auth.Properties == nil || auth.Properties.AuthMethod != "token" {
				t.Fatalf("expected auth method token, got %+v", auth.Properties)
			}
			if calls == 1 {
				return &packets.Auth{
					ReasonCode: packets.AuthContinueAuthentication,
					Properties: &packets.AuthProperties{
						AuthMethod: "token",
						AuthData:   []byte("challenge"),
					},
				}, nil
			}
			return &packets.Auth{
				ReasonCode: packets.AuthSuccess,
				Properties: &packets.AuthProperties{
					AuthMethod: "token",
					AuthData:   []byte("ok"),
				},
			}, nil
		},
	}

	sessionCenter := &fakeSessionCenter{}
	subCenter := &recordingSubCenter{}
	c := NewClient(
		c1,
		WithPlugin(plugins),
		WithSessionCenter(sessionCenter),
		WithSubCenter(subCenter),
		WithStateRouter(newTestInProcessStateRouter(t, sessionCenter, subCenter)),
		WithClientManager(NewManager()),
	)
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	defer c.cancel(nil)

	handler := NewClientHandler(c)
	connectErr := make(chan error, 1)
	go func() {
		connectErr <- handler.handleConnect(&packets.Connect{
			ProtocolName:    "MQTT",
			ProtocolVersion: 5,
			ClientID:        "client-auth",
			CleanStart:      true,
			KeepAlive:       30,
			Properties: &packets.ConnectProperties{
				AuthMethod: "token",
				AuthData:   []byte("hello"),
			},
		})
	}()

	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	first, err := wire.Decode(c2, wire.DecodeOptions{})
	if err != nil {
		t.Fatalf("read initial auth response: %v", err)
	}
	if first.FixedHeader.Type != packets.AUTH {
		t.Fatalf("expected AUTH while authentication continues, got %s", first.PacketType())
	}
	authResp := first.Content.(*packets.Auth)
	if authResp.ReasonCode != packets.AuthContinueAuthentication {
		t.Fatalf("expected Continue Authentication, got 0x%X", authResp.ReasonCode)
	}
	if err := <-connectErr; err != nil {
		t.Fatalf("handleConnect: %v", err)
	}

	authErr := make(chan error, 1)
	go func() {
		authErr <- handler.handleAuth(context.Background(), &packets.Auth{
			ReasonCode: packets.AuthContinueAuthentication,
			Properties: &packets.AuthProperties{
				AuthMethod: "token",
				AuthData:   []byte("answer"),
			},
		})
	}()

	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	second, err := wire.Decode(c2, wire.DecodeOptions{})
	if err != nil {
		t.Fatalf("read connack: %v", err)
	}
	if second.FixedHeader.Type != packets.CONNACK {
		t.Fatalf("expected CONNACK after auth success, got %s", second.PacketType())
	}
	connAck := second.Content.(*packets.ConnAck)
	if connAck.ReasonCode != packets.ConnAckSuccess {
		t.Fatalf("expected CONNACK success, got 0x%X", connAck.ReasonCode)
	}
	if connAck.Properties == nil || connAck.Properties.AuthMethod != "token" || string(connAck.Properties.AuthData) != "ok" {
		t.Fatalf("expected CONNACK auth properties token/ok, got %+v", connAck.Properties)
	}
	if err := <-authErr; err != nil {
		t.Fatalf("handleAuth: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected auth plugin called twice, got %d", calls)
	}
}

func TestHandlePacketAllowsDisconnectDuringInitialEnhancedAuth(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	c := NewClient(c1)
	c.ID = "client-auth"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.enhancedAuthMethod = "token"
	c.enhancedAuthState = enhancedAuthAuthenticating

	cp := packets.NewControlPacket(packets.DISCONNECT)
	cp.Content = &packets.Disconnect{ReasonCode: 0}

	err := NewClientHandler(c).HandlePacket(context.Background(), cp, c)
	if err != nil {
		t.Fatalf("expected DISCONNECT during enhanced auth to be accepted, got %v", err)
	}

	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	if serverPacket, readErr := wire.Decode(c2, wire.DecodeOptions{}); readErr == nil {
		t.Fatalf("server must not send DISCONNECT before successful CONNACK, got %s", serverPacket.PacketType())
	}
}

func TestHandlePacketAllowsPingreqDuringEnhancedReauthentication(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	c := NewClient(c1)
	c.ID = "client-auth-reauth"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.connAckAccepted.Store(true)
	c.enhancedAuthMethod = "token"
	c.enhancedAuthState = enhancedAuthReauthenticating

	readCh := make(chan *packets.ControlPacket, 1)
	errCh := make(chan error, 1)
	go func() {
		_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
		cp, readErr := wire.Decode(c2, wire.DecodeOptions{})
		if readErr != nil {
			errCh <- readErr
			return
		}
		readCh <- cp
	}()

	err := NewClientHandler(c).HandlePacket(context.Background(), packets.NewControlPacket(packets.PINGREQ), c)
	if err != nil {
		t.Fatalf("expected PINGREQ during enhanced re-authentication to be accepted, got %v", err)
	}

	var cp *packets.ControlPacket
	select {
	case cp = <-readCh:
	case readErr := <-errCh:
		t.Fatalf("read response: %v", readErr)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for PINGRESP")
	}
	if cp.FixedHeader.Type != packets.PINGRESP {
		t.Fatalf("expected PINGRESP, got %s", cp.PacketType())
	}
}

func TestHandleAuthDuringInitialEnhancedAuthRequiresContinueReason(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	c := NewClient(c1)
	c.ID = "client-auth"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.enhancedAuthMethod = "token"
	c.enhancedAuthState = enhancedAuthAuthenticating

	err := NewClientHandler(c).handleAuth(context.Background(), &packets.Auth{
		ReasonCode: packets.AuthSuccess,
		Properties: &packets.AuthProperties{AuthMethod: "token"},
	})
	if !errors.Is(err, ErrProtocolError) {
		t.Fatalf("expected protocol error, got %v", err)
	}

	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	if serverPacket, readErr := wire.Decode(c2, wire.DecodeOptions{}); readErr == nil {
		t.Fatalf("server must not send DISCONNECT before successful CONNACK, got %s", serverPacket.PacketType())
	}
}

func TestHandleAuthAfterConnectRequiresReauthenticateReason(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	c := NewClient(c1)
	c.ID = "client-auth"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.enhancedAuthMethod = "token"
	c.enhancedAuthState = enhancedAuthAuthenticated
	c.connAckAccepted.Store(true)

	errCh := make(chan error, 1)
	go func() {
		errCh <- NewClientHandler(c).handleAuth(context.Background(), &packets.Auth{
			ReasonCode: packets.AuthContinueAuthentication,
			Properties: &packets.AuthProperties{AuthMethod: "token"},
		})
	}()

	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	cp, readErr := wire.Decode(c2, wire.DecodeOptions{})
	if readErr != nil {
		t.Fatalf("expected DISCONNECT after successful CONNACK, got read error: %v", readErr)
	}
	disc, ok := cp.Content.(*packets.Disconnect)
	if !ok {
		t.Fatalf("expected DISCONNECT, got %T", cp.Content)
	}
	if disc.ReasonCode != packets.DisconnectProtocolError {
		t.Fatalf("expected Protocol Error, got 0x%X", disc.ReasonCode)
	}
	if err := <-errCh; !errors.Is(err, ErrProtocolError) {
		t.Fatalf("expected protocol error, got %v", err)
	}
}

func TestEnhancedAuthRejectsConnectWhenNoAuthHandlerConfigured(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	c := NewClient(
		c1,
		WithPlugin(&plugin.Plugins{}),
		WithSessionCenter(&fakeSessionCenter{}),
		WithSubCenter(&recordingSubCenter{}),
		WithClientManager(NewManager()),
	)
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	defer c.cancel(nil)

	errCh := make(chan error, 1)
	go func() {
		errCh <- NewClientHandler(c).handleConnect(&packets.Connect{
			ProtocolName:    "MQTT",
			ProtocolVersion: 5,
			ClientID:        "client-auth-missing",
			CleanStart:      true,
			KeepAlive:       30,
			Properties: &packets.ConnectProperties{
				AuthMethod: "token",
				AuthData:   []byte("hello"),
			},
		})
	}()

	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	cp, err := wire.Decode(c2, wire.DecodeOptions{})
	if err != nil {
		t.Fatalf("read connack: %v", err)
	}
	connAck, ok := cp.Content.(*packets.ConnAck)
	if !ok {
		t.Fatalf("expected CONNACK, got %T", cp.Content)
	}
	if connAck.ReasonCode != packets.ConnAckBadAuthenticationMethod {
		t.Fatalf("expected Bad Authentication Method, got 0x%X", connAck.ReasonCode)
	}

	if err := <-errCh; err != ErrAuthHandlerNotSet {
		t.Fatalf("expected auth handler error, got %v", err)
	}
}

func TestHandleAuthDuringInitialEnhancedAuthFailureSendsConnAck(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	c := NewClient(c1)
	c.ID = "client-auth-fail"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	defer c.cancel(nil)
	c.enhancedAuthMethod = "token"
	c.enhancedAuthState = enhancedAuthAuthenticating
	c.pendingEnhancedAuthConnect = &packets.Connect{
		ProtocolName:    "MQTT",
		ProtocolVersion: 5,
		ClientID:        "client-auth-fail",
		CleanStart:      true,
		Properties: &packets.ConnectProperties{
			AuthMethod: "token",
		},
	}

	handler := NewClientHandler(c)
	errCh := make(chan error, 1)
	go func() {
		errCh <- handler.handleAuthDuringConnect(context.Background(), &packets.Auth{
			ReasonCode: packets.AuthContinueAuthentication,
			Properties: &packets.AuthProperties{AuthMethod: "token"},
		}, time.Now())
	}()

	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	cp, err := wire.Decode(c2, wire.DecodeOptions{})
	if err != nil {
		t.Fatalf("read CONNACK: %v", err)
	}
	connAck, ok := cp.Content.(*packets.ConnAck)
	if !ok {
		t.Fatalf("expected CONNACK, got %T", cp.Content)
	}
	if connAck.ReasonCode != packets.ConnAckBadAuthenticationMethod {
		t.Fatalf("expected Bad Authentication Method, got 0x%X", connAck.ReasonCode)
	}
	if err := <-errCh; !errors.Is(err, ErrAuthHandlerNotSet) {
		t.Fatalf("expected auth handler error, got %v", err)
	}
}

func TestConnectAdmissionRedirectsWithServerReference(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	c := NewClient(
		c1,
		WithConnectAdmission(func(ctx context.Context, clientID string, connectPacket *packets.Connect) ConnectAdmissionResult {
			return ConnectAdmissionResult{
				ReasonCode:      packets.ConnAckUseAnotherServer,
				ReasonString:    "redirect",
				ServerReference: "mqtt://broker-b",
			}
		}),
	)
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	defer c.cancel(nil)

	errCh := make(chan error, 1)
	go func() {
		errCh <- NewClientHandler(c).handleConnect(&packets.Connect{
			ProtocolName:    "MQTT",
			ProtocolVersion: 5,
			ClientID:        "client-redirect",
			CleanStart:      true,
			Properties:      &packets.ConnectProperties{},
		})
	}()

	_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	cp, err := wire.Decode(c2, wire.DecodeOptions{})
	if err != nil {
		t.Fatalf("read CONNACK: %v", err)
	}
	connAck, ok := cp.Content.(*packets.ConnAck)
	if !ok {
		t.Fatalf("expected CONNACK, got %T", cp.Content)
	}
	if connAck.ReasonCode != packets.ConnAckUseAnotherServer {
		t.Fatalf("expected Use Another Server, got 0x%X", connAck.ReasonCode)
	}
	if connAck.Properties == nil || connAck.Properties.ServerReference != "mqtt://broker-b" {
		t.Fatalf("expected ServerReference mqtt://broker-b, got %+v", connAck.Properties)
	}
	if err := <-errCh; !errors.Is(err, ErrProtocolError) {
		t.Fatalf("expected protocol error, got %v", err)
	}
}
