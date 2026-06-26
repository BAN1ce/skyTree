package client

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/internal/broker/client/rate"
	"github.com/BAN1ce/skyTree/internal/broker/delivery"
	"github.com/BAN1ce/skyTree/internal/broker/plugin"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/message/serializer"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/wire"
	"github.com/BAN1ce/skyTree/proto/proto_session"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

func TestRestoreOutgoingInflight_WaitingPubcomp_RetransmitsPubrelWithDup(t *testing.T) {
	initClientPackageConfig(t)
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	msgID := uuid.MustParse("00000000-0000-0000-0000-000000000010")
	taskID := uuid.MustParse("00000000-0000-0000-0000-000000000011")

	cs := &fakeCursorStore{
		readTasksFn: func(ctx context.Context, clientID string, lastTS time.Time, lastTaskID uuid.UUID, limit int) ([]*store.DeliveryTask, error) {
			return []*store.DeliveryTask{
				{
					TS:                time.Unix(1, 0),
					TaskID:            taskID,
					ClientID:          clientID,
					MessageID:         msgID,
					RetainAsPublished: true,
				},
			}, nil
		},
	}

	cl := NewClient(c1)
	cl.ID = "test-client"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	cl.component.plugin = &plugin.Plugins{}
	cl.component.deliveryCursorStore = delivery.CursorStore(cs)

	sess := &proto_session.Session{
		ClientID: cl.ID,
		UnfinishedMessages: []*proto_session.UnfinishedMessage{
			{
				MessageID:    msgID.String(),
				PacketID:     7,
				Qos:          2,
				State:        proto_session.UnfinishedMessage_WAITING_PUBCOMP,
				IsOutgoing:   true,
				FirstPubTime: time.Now().UnixMicro(),
			},
		},
	}

	errCh := make(chan error, 1)
	go func() {
		cl.restoreOutgoingInflightFromSession(context.Background(), sess)
		errCh <- nil
	}()

	_ = c2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	got, err := wire.Decode(c2, wire.DecodeOptions{})
	if err != nil {
		t.Fatalf("read packet: %v", err)
	}
	if got.Type != packets.PUBREL {
		t.Fatalf("expected PUBREL, got %d", got.Type)
	}
	if got.Flags != 0x2 {
		t.Fatalf("expected PUBREL flags=0x2, got 0x%X", got.Flags)
	}
	pr, ok := got.Content.(*packets.Pubrel)
	if !ok || pr == nil {
		t.Fatalf("expected Pubrel content, got %T", got.Content)
	}
	if pr.PacketID != 7 {
		t.Fatalf("expected PacketID=7, got %d", pr.PacketID)
	}
	_ = <-errCh
}

func TestRestoreOutgoingInflight_RestoresAllOutgoingMessages(t *testing.T) {
	initClientPackageConfig(t)
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	msgID1 := uuid.MustParse("00000000-0000-0000-0000-000000000101")
	msgID2 := uuid.MustParse("00000000-0000-0000-0000-000000000102")
	taskID1 := uuid.MustParse("00000000-0000-0000-0000-000000000111")
	taskID2 := uuid.MustParse("00000000-0000-0000-0000-000000000112")

	cs := &fakeCursorStore{
		readTasksFn: func(ctx context.Context, clientID string, lastTS time.Time, lastTaskID uuid.UUID, limit int) ([]*store.DeliveryTask, error) {
			return []*store.DeliveryTask{
				{
					TS:                time.Unix(1, 0),
					TaskID:            taskID1,
					ClientID:          clientID,
					MessageID:         msgID1,
					RetainAsPublished: true,
				},
				{
					TS:                time.Unix(2, 0),
					TaskID:            taskID2,
					ClientID:          clientID,
					MessageID:         msgID2,
					RetainAsPublished: true,
				},
			}, nil
		},
	}

	cl := NewClient(c1)
	cl.ID = "test-client"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	cl.component.plugin = &plugin.Plugins{}
	cl.component.deliveryCursorStore = delivery.CursorStore(cs)

	sess := &proto_session.Session{
		ClientID: cl.ID,
		UnfinishedMessages: []*proto_session.UnfinishedMessage{
			{
				MessageID:    msgID1.String(),
				PacketID:     7,
				Qos:          2,
				State:        proto_session.UnfinishedMessage_WAITING_PUBCOMP,
				IsOutgoing:   true,
				FirstPubTime: time.Unix(1, 0).UnixMicro(),
			},
			{
				MessageID:    msgID2.String(),
				PacketID:     8,
				Qos:          2,
				State:        proto_session.UnfinishedMessage_WAITING_PUBCOMP,
				IsOutgoing:   true,
				FirstPubTime: time.Unix(2, 0).UnixMicro(),
			},
		},
	}

	errCh := make(chan error, 1)
	go func() {
		cl.restoreOutgoingInflightFromSession(context.Background(), sess)
		errCh <- nil
	}()

	for _, wantPacketID := range []uint16{7, 8} {
		_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
		got, err := wire.Decode(c2, wire.DecodeOptions{})
		if err != nil {
			t.Fatalf("read restored packet %d: %v", wantPacketID, err)
		}
		pubrel, ok := got.Content.(*packets.Pubrel)
		if !ok {
			t.Fatalf("expected PUBREL, got %T", got.Content)
		}
		if pubrel.PacketID != wantPacketID {
			t.Fatalf("expected packet id %d, got %d", wantPacketID, pubrel.PacketID)
		}
	}
	_ = <-errCh
	if got := cl.outgoingInflight.Len(); got != 2 {
		t.Fatalf("expected two restored inflight entries, got %d", got)
	}
}

func TestRestoreOutgoingInflight_DoesNotBlockWhenRestoredCountExceedsReceiveMaximum(t *testing.T) {
	initClientPackageConfig(t)
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	msgID1 := uuid.MustParse("00000000-0000-0000-0000-000000000201")
	msgID2 := uuid.MustParse("00000000-0000-0000-0000-000000000202")
	taskID1 := uuid.MustParse("00000000-0000-0000-0000-000000000211")
	taskID2 := uuid.MustParse("00000000-0000-0000-0000-000000000212")

	cs := &fakeCursorStore{
		readTasksFn: func(ctx context.Context, clientID string, lastTS time.Time, lastTaskID uuid.UUID, limit int) ([]*store.DeliveryTask, error) {
			return []*store.DeliveryTask{
				{TS: time.Unix(1, 0), TaskID: taskID1, ClientID: clientID, MessageID: msgID1},
				{TS: time.Unix(2, 0), TaskID: taskID2, ClientID: clientID, MessageID: msgID2},
			}, nil
		},
	}

	cl := NewClient(&bufferConn{})
	cl.ID = "test-client"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	cl.publishBucket = rate.NewBucket(1)
	cl.component.deliveryCursorStore = delivery.CursorStore(cs)

	sess := &proto_session.Session{
		ClientID: cl.ID,
		UnfinishedMessages: []*proto_session.UnfinishedMessage{
			{
				MessageID:    msgID1.String(),
				PacketID:     7,
				Qos:          2,
				State:        proto_session.UnfinishedMessage_WAITING_PUBCOMP,
				IsOutgoing:   true,
				FirstPubTime: time.Unix(1, 0).UnixMicro(),
			},
			{
				MessageID:    msgID2.String(),
				PacketID:     8,
				Qos:          2,
				State:        proto_session.UnfinishedMessage_WAITING_PUBCOMP,
				IsOutgoing:   true,
				FirstPubTime: time.Unix(2, 0).UnixMicro(),
			},
		},
	}

	done := make(chan struct{})
	go func() {
		cl.restoreOutgoingInflightFromSession(context.Background(), sess)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("restore should not block when persisted inflight exceeds current Receive Maximum")
	}
	if got := cl.outgoingInflight.Len(); got != 2 {
		t.Fatalf("expected both inflight entries to be restored, got %d", got)
	}
	if got := cl.publishBucket.RemainingToken(); got != 0 {
		t.Fatalf("expected restored first packet to occupy the only receive maximum token, got %d", got)
	}
}

func TestRestoreOutgoingInflight_RestoresQoS1ReplayCursor(t *testing.T) {
	initClientPackageConfig(t)
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	taskID := uuid.MustParse("00000000-0000-0000-0000-000000000220")
	taskTS := time.UnixMicro(123456)
	generation := int64(3)
	cs := &fakeCursorStore{
		readCursorFn: func(ctx context.Context, clientID string) (*store.DeliveryCursor, error) {
			return &store.DeliveryCursor{
				ClientID:   clientID,
				Generation: generation,
				LastTS:     time.UnixMicro(100),
				LastTaskID: uuid.Nil,
			}, nil
		},
	}

	cl := NewClient(&bufferConn{})
	cl.ID = "test-client"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	cl.component.deliveryCursorStore = delivery.CursorStore(cs)

	cl.restoreOutgoingInflightFromSession(context.Background(), &proto_session.Session{
		ClientID: cl.ID,
		OutgoingReplayCursor: &proto_session.OutgoingReplayCursor{
			TaskUnixMicro: taskTS.UnixMicro(),
			TaskID:        taskID.String(),
			Generation:    generation,
		},
	})

	if cl.outgoingReplayCursor == nil {
		t.Fatal("expected qos1 replay cursor to be restored")
	}
	if cl.outgoingReplayCursor.TaskID != taskID {
		t.Fatalf("expected task id %s, got %s", taskID, cl.outgoingReplayCursor.TaskID)
	}
	if !cl.outgoingReplayCursor.TaskTS.Equal(taskTS) {
		t.Fatalf("expected task ts %s, got %s", taskTS, cl.outgoingReplayCursor.TaskTS)
	}
	lastTS, lastTask := cl.loadClientDeliveryCursor(delivery.CursorStore(cs))
	if !lastTS.Equal(taskTS.Add(-time.Microsecond)) {
		t.Fatalf("expected replay start %s, got %s", taskTS.Add(-time.Microsecond), lastTS)
	}
	if lastTask != uuid.Nil {
		t.Fatalf("expected nil last task for replay start, got %s", lastTask)
	}
}

func TestRestoreOutgoingInflight_IgnoresOrdinaryQoS1Unfinished(t *testing.T) {
	initClientPackageConfig(t)
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	cl := NewClient(&bufferConn{})
	cl.ID = "test-client"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	cl.component.deliveryCursorStore = delivery.CursorStore(&fakeCursorStore{})

	cl.restoreOutgoingInflightFromSession(context.Background(), &proto_session.Session{
		ClientID: cl.ID,
		UnfinishedMessages: []*proto_session.UnfinishedMessage{
			{
				MessageID:  uuid.NewString(),
				PacketID:   7,
				Qos:        1,
				State:      proto_session.UnfinishedMessage_WAITING_PUBACK,
				IsOutgoing: true,
			},
		},
	})

	if got := cl.outgoingInflight.Len(); got != 0 {
		t.Fatalf("expected ordinary qos1 unfinished to be ignored, got %d inflight entries", got)
	}
}

func TestRestoreOutgoingInflight_WaitingPubrec_RetransmitsPublishDupSamePacketID(t *testing.T) {
	initClientPackageConfig(t)
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	msgID := uuid.MustParse("00000000-0000-0000-0000-000000000020")
	taskID := uuid.MustParse("00000000-0000-0000-0000-000000000021")

	// Build message payload with QoS0; retransmit should override to QoS2 and set DUP+PacketID.
	p := &packets.Publish{Topic: "t/a", QoS: 0, Retain: true, Payload: []byte("x")}
	m := &brokerpublish.Message{SendClientID: "publisher"}
	m.SetPublish(p)
	raw, err := serializer.Serializer.Encode(m)
	if err != nil {
		t.Fatalf("encode message: %v", err)
	}

	cs := &fakeCursorStore{
		readTasksFn: func(ctx context.Context, clientID string, lastTS time.Time, lastTaskID uuid.UUID, limit int) ([]*store.DeliveryTask, error) {
			return []*store.DeliveryTask{
				{
					TS:                time.Unix(1, 0),
					TaskID:            taskID,
					ClientID:          clientID,
					MessageID:         msgID,
					RetainAsPublished: true,
				},
			}, nil
		},
		loadMessagePayloadFn: func(ctx context.Context, messageID uuid.UUID) ([]byte, error) { return raw, nil },
	}

	cl := NewClient(c1)
	cl.ID = "test-client"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	cl.component.plugin = &plugin.Plugins{}
	cl.component.deliveryCursorStore = delivery.CursorStore(cs)

	sess := &proto_session.Session{
		ClientID: cl.ID,
		UnfinishedMessages: []*proto_session.UnfinishedMessage{
			{
				MessageID:    msgID.String(),
				PacketID:     42,
				Qos:          2,
				State:        proto_session.UnfinishedMessage_WAITING_PUBREC,
				IsOutgoing:   true,
				FirstPubTime: time.Now().UnixMicro(),
			},
		},
	}

	errCh := make(chan error, 1)
	go func() {
		cl.restoreOutgoingInflightFromSession(context.Background(), sess)
		errCh <- nil
	}()

	_ = c2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	got, err := wire.Decode(c2, wire.DecodeOptions{})
	if err != nil {
		t.Fatalf("read packet: %v", err)
	}
	if got.Type != packets.PUBLISH {
		t.Fatalf("expected PUBLISH, got %d", got.Type)
	}
	pub, ok := got.Content.(*packets.Publish)
	if !ok || pub == nil {
		t.Fatalf("expected Publish content, got %T", got.Content)
	}
	if !pub.Duplicate {
		t.Fatalf("expected DUP=true on retransmit PUBLISH")
	}
	if pub.PacketID != 42 {
		t.Fatalf("expected PacketID=42, got %d", pub.PacketID)
	}
	if pub.QoS != 2 {
		t.Fatalf("expected QoS=2, got %d", pub.QoS)
	}
	_ = <-errCh
}

func TestDeliveryRunner_WithRestoredInflight_SkipsInitialReadTasks(t *testing.T) {
	initClientPackageConfig(t)
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	cs := &fakeCursorStore{
		readTasksFn: func(ctx context.Context, clientID string, lastTS time.Time, lastTaskID uuid.UUID, limit int) ([]*store.DeliveryTask, error) {
			return nil, nil
		},
	}

	cl := NewClient(c1)
	cl.ID = "test-client"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	cl.component.deliveryCursorStore = delivery.CursorStore(cs)

	// Simulate restored inflight.
	if cl.outgoingInflight != nil {
		cl.outgoingInflight.Put(&outgoingInflightEntry{
			PacketID:     1,
			QoS:          1,
			TaskTS:       time.Unix(1, 0),
			TaskID:       uuid.MustParse("00000000-0000-0000-0000-000000000030"),
			MessageID:    uuid.MustParse("00000000-0000-0000-0000-000000000031"),
			FirstPubTime: time.Now(),
			LastSendTime: time.Now(),
			State:        outInflightWaitingPubAck,
		})
	}

	go cl.runClientDeliveryRunner()

	// Give it a moment; initial probe should be skipped when inflight exists.
	time.Sleep(100 * time.Millisecond)
	if cs.readTasksCalls.Load() != 0 {
		t.Fatalf("expected 0 initial ReadTasks call when inflight exists, got %d", cs.readTasksCalls.Load())
	}

	cl.cancel(nil)
	_ = c2.Close()
}
