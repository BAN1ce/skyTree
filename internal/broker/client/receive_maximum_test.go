package client

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/broker/client/rate"
	"github.com/BAN1ce/skyTree/internal/broker/delivery"
	"github.com/BAN1ce/skyTree/logger"
	clientcap "github.com/BAN1ce/skyTree/pkg/brokerapi/clientcap"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	"github.com/BAN1ce/skyTree/pkg/metric"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/wire"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type bufferConn struct {
	bytes.Buffer
}

func (c *bufferConn) Close() error                     { return nil }
func (c *bufferConn) LocalAddr() net.Addr              { return dummyAddr("local") }
func (c *bufferConn) RemoteAddr() net.Addr             { return dummyAddr("remote") }
func (c *bufferConn) SetDeadline(time.Time) error      { return nil }
func (c *bufferConn) SetReadDeadline(time.Time) error  { return nil }
func (c *bufferConn) SetWriteDeadline(time.Time) error { return nil }

type dummyAddr string

func (a dummyAddr) Network() string { return string(a) }
func (a dummyAddr) String() string  { return string(a) }

func TestWritePubrecDoesNotConsumeOutboundReceiveMaximumToken(t *testing.T) {
	_ = func() error { _, err := config.Load("../../../etc/config.yaml"); return err }()
	c := NewClient(&bufferConn{})
	c.ctx = context.Background()
	c.publishBucket = rate.NewBucket(1)

	cp := packets.NewControlPacket(packets.PUBREC)
	cp.Content = &packets.Pubrec{PacketID: 10}
	if err := c.write(&clientcap.WritePacket{Packet: cp}); err != nil {
		t.Fatalf("write pubrec: %v", err)
	}

	if got := c.publishBucket.RemainingToken(); got != 1 {
		t.Fatalf("PUBREC must not consume outbound receive maximum token, remaining=%d", got)
	}
}

func TestHandlePubAckDoesNotReleaseTokenWithoutMatchingInflight(t *testing.T) {
	_ = func() error { _, err := config.Load("../../../etc/config.yaml"); return err }()
	c := NewClient(&bufferConn{})
	c.ID = "client-a"
	c.ctx = context.Background()
	c.publishBucket = rate.NewBucket(1)
	c.publishBucket.GetToken(c.ctx)

	if err := NewClientHandler(c).handlePubAck(context.Background(), &packets.Puback{PacketID: 77}); err != nil {
		t.Fatalf("handle puback: %v", err)
	}

	if got := c.publishBucket.RemainingToken(); got != 0 {
		t.Fatalf("unmatched PUBACK must not release receive maximum token, remaining=%d", got)
	}
}

func TestHandlePubAckWithErrorReasonAdvancesCursorAndReleasesToken(t *testing.T) {
	_ = func() error { _, err := config.Load("../../../etc/config.yaml"); return err }()
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}
	c := NewClient(&bufferConn{})
	c.ID = "client-a"
	c.ctx = context.Background()
	c.publishBucket = rate.NewBucket(1)
	c.publishBucket.GetToken(c.ctx)

	var advanceCalls atomic.Int64
	c.component.deliveryCursorStore = delivery.CursorStore(&fakeCursorStore{
		advanceCursorFn: func(context.Context, store.DeliveryCursor) error {
			advanceCalls.Add(1)
			return nil
		},
	})
	c.outgoingInflight.Put(&outgoingInflightEntry{
		PacketID:      7,
		QoS:           1,
		TaskTS:        time.Now(),
		TaskID:        uuid.New(),
		State:         outInflightWaitingPubAck,
		FlowTokenHeld: true,
	})

	if err := NewClientHandler(c).handlePubAck(context.Background(), &packets.Puback{
		PacketID:   7,
		ReasonCode: packets.PubackNotAuthorized,
	}); err != nil {
		t.Fatalf("handle puback: %v", err)
	}

	if advanceCalls.Load() != 1 {
		t.Fatalf("negative PUBACK must advance delivery cursor once, got %d", advanceCalls.Load())
	}
	if got := c.publishBucket.RemainingToken(); got != 1 {
		t.Fatalf("negative PUBACK must release receive maximum token, remaining=%d", got)
	}
	if _, ok := c.outgoingInflight.Get(7); ok {
		t.Fatalf("negative PUBACK must remove terminal outgoing inflight")
	}
}

func TestHandlePubAckRecordsAckDelayMetrics(t *testing.T) {
	_ = func() error { _, err := config.Load("../../../etc/config.yaml"); return err }()
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	tests := []struct {
		name       string
		reasonCode byte
		result     string
	}{
		{
			name:   "success",
			result: "success",
		},
		{
			name:       "negative",
			reasonCode: packets.PubackNotAuthorized,
			result:     "negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewClient(&bufferConn{})
			c.ID = "client-a"
			c.ctx = context.Background()
			c.component.deliveryCursorStore = delivery.CursorStore(&fakeCursorStore{})
			c.outgoingInflight.Put(&outgoingInflightEntry{
				PacketID:     17,
				QoS:          1,
				TaskTS:       time.Now().Add(-time.Second),
				TaskID:       uuid.New(),
				FirstPubTime: time.Now().Add(-time.Second),
				State:        outInflightWaitingPubAck,
			})
			before := clientHistogramSampleCount(t, metric.DeliveryAckDelaySeconds, map[string]string{
				"path":   "normal",
				"qos":    "1",
				"result": tt.result,
			})

			if err := NewClientHandler(c).handlePubAck(context.Background(), &packets.Puback{
				PacketID:   17,
				ReasonCode: tt.reasonCode,
			}); err != nil {
				t.Fatalf("handle puback: %v", err)
			}

			assertClientHistogramSampleCount(t, metric.DeliveryAckDelaySeconds, map[string]string{
				"path":   "normal",
				"qos":    "1",
				"result": tt.result,
			}, before+1)
		})
	}
}

func TestHandlePubRecWithErrorReasonCompletesDeliveryWithoutPubRel(t *testing.T) {
	_ = func() error { _, err := config.Load("../../../etc/config.yaml"); return err }()
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}
	conn := &bufferConn{}
	c := NewClient(conn)
	c.ID = "client-a"
	c.ctx = context.Background()
	c.publishBucket = rate.NewBucket(1)
	c.publishBucket.GetToken(c.ctx)
	var advanceCalls atomic.Int64
	c.component.deliveryCursorStore = delivery.CursorStore(&fakeCursorStore{
		advanceCursorFn: func(context.Context, store.DeliveryCursor) error {
			advanceCalls.Add(1)
			return nil
		},
	})
	c.outgoingInflight.Put(&outgoingInflightEntry{
		PacketID:      8,
		QoS:           2,
		TaskTS:        time.Now(),
		TaskID:        uuid.New(),
		State:         outInflightWaitingPubRec,
		FlowTokenHeld: true,
	})

	if err := NewClientHandler(c).handlePubRec(context.Background(), &packets.Pubrec{
		PacketID:   8,
		ReasonCode: packets.PubrecNotAuthorized,
	}); err != nil {
		t.Fatalf("handle pubrec: %v", err)
	}

	if conn.Len() != 0 {
		t.Fatalf("negative PUBREC must not send PUBREL, wrote %d bytes", conn.Len())
	}
	if advanceCalls.Load() != 1 {
		t.Fatalf("negative PUBREC must advance delivery cursor once, got %d", advanceCalls.Load())
	}
	if got := c.publishBucket.RemainingToken(); got != 1 {
		t.Fatalf("negative PUBREC must release receive maximum token, remaining=%d", got)
	}
	if _, ok := c.outgoingInflight.Get(8); ok {
		t.Fatalf("negative PUBREC must remove terminal outgoing inflight")
	}
}

func TestHandlePubCompWithErrorReasonAdvancesCursorAndReleasesToken(t *testing.T) {
	_ = func() error { _, err := config.Load("../../../etc/config.yaml"); return err }()
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}
	c := NewClient(&bufferConn{})
	c.ID = "client-a"
	c.ctx = context.Background()
	c.publishBucket = rate.NewBucket(1)
	c.publishBucket.GetToken(c.ctx)

	var advanceCalls atomic.Int64
	c.component.deliveryCursorStore = delivery.CursorStore(&fakeCursorStore{
		advanceCursorFn: func(context.Context, store.DeliveryCursor) error {
			advanceCalls.Add(1)
			return nil
		},
	})
	c.outgoingInflight.Put(&outgoingInflightEntry{
		PacketID:      9,
		QoS:           2,
		TaskTS:        time.Now(),
		TaskID:        uuid.New(),
		State:         outInflightWaitingPubComp,
		FlowTokenHeld: true,
	})

	if err := NewClientHandler(c).handlePubComp(context.Background(), &packets.Pubcomp{
		PacketID:   9,
		ReasonCode: packets.PubcompPacketIdentifierNotFound,
	}); err != nil {
		t.Fatalf("handle pubcomp: %v", err)
	}

	if advanceCalls.Load() != 1 {
		t.Fatalf("negative PUBCOMP must advance delivery cursor once, got %d", advanceCalls.Load())
	}
	if got := c.publishBucket.RemainingToken(); got != 1 {
		t.Fatalf("negative PUBCOMP must release receive maximum token, remaining=%d", got)
	}
	if _, ok := c.outgoingInflight.Get(9); ok {
		t.Fatalf("negative PUBCOMP must remove terminal outgoing inflight")
	}
}

func TestHandlePublishDisconnectsWhenQoS1ReceiveMaximumAlreadyFull(t *testing.T) {
	_ = func() error { _, err := config.Load("../../../etc/config.yaml"); return err }()
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	c := NewClient(c1)
	c.ID = "client-a"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.serverReceiveMaximum = 1
	c.incomingQoS1Inflight[100] = struct{}{}

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

	err := NewClientHandler(c).handlePublish(context.Background(), &packets.Publish{
		Topic:    "receive/max",
		QoS:      1,
		PacketID: 10,
	})
	if err == nil {
		t.Fatalf("expected receive maximum protocol error")
	}

	select {
	case cp := <-readCh:
		disc, ok := cp.Content.(*packets.Disconnect)
		if !ok {
			t.Fatalf("expected DISCONNECT, got %T", cp.Content)
		}
		if disc.ReasonCode != packets.DisconnectReceiveMaximumExceeded {
			t.Fatalf("expected Receive Maximum Exceeded, got 0x%X", disc.ReasonCode)
		}
	case err := <-errCh:
		t.Fatalf("read packet error: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for DISCONNECT")
	}
}
