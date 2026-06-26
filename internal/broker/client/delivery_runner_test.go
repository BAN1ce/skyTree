package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/broker/client/rate"
	"github.com/BAN1ce/skyTree/internal/broker/delivery"
	"github.com/BAN1ce/skyTree/internal/broker/plugin"
	shared_manager "github.com/BAN1ce/skyTree/internal/broker/sharedsubscription/manager"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/message/serializer"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	"github.com/BAN1ce/skyTree/pkg/metric"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/wire"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/rs/zerolog"
)

type callbackConn struct {
	bytes.Buffer
	onFirstWrite func()
	called       atomic.Bool
}

func (c *callbackConn) Write(p []byte) (int, error) {
	if c.onFirstWrite != nil && c.called.CompareAndSwap(false, true) {
		c.onFirstWrite()
	}
	return c.Buffer.Write(p)
}

func (c *callbackConn) Close() error                     { return nil }
func (c *callbackConn) LocalAddr() net.Addr              { return dummyAddr("local") }
func (c *callbackConn) RemoteAddr() net.Addr             { return dummyAddr("remote") }
func (c *callbackConn) SetDeadline(time.Time) error      { return nil }
func (c *callbackConn) SetReadDeadline(time.Time) error  { return nil }
func (c *callbackConn) SetWriteDeadline(time.Time) error { return nil }

type fakeCursorStore struct {
	readTasksCalls atomic.Int64

	readCursorFn         func(ctx context.Context, clientID string) (*store.DeliveryCursor, error)
	readTasksFn          func(ctx context.Context, clientID string, lastTS time.Time, lastTaskID uuid.UUID, limit int) ([]*store.DeliveryTask, error)
	loadMessagePayloadFn func(ctx context.Context, messageID uuid.UUID) ([]byte, error)
	advanceCursorFn      func(ctx context.Context, cursor store.DeliveryCursor) error
}

func (f *fakeCursorStore) ReadCursor(ctx context.Context, clientID string) (*store.DeliveryCursor, error) {
	if f.readCursorFn != nil {
		return f.readCursorFn(ctx, clientID)
	}
	return &store.DeliveryCursor{ClientID: clientID, LastTS: time.UnixMicro(0), LastTaskID: uuid.Nil}, nil
}
func (f *fakeCursorStore) AdvanceCursor(ctx context.Context, cursor store.DeliveryCursor) error {
	if f.advanceCursorFn != nil {
		return f.advanceCursorFn(ctx, cursor)
	}
	return nil
}
func (f *fakeCursorStore) ReadTasks(ctx context.Context, clientID string, lastTS time.Time, lastTaskID uuid.UUID, limit int) ([]*store.DeliveryTask, error) {
	f.readTasksCalls.Add(1)
	if f.readTasksFn != nil {
		return f.readTasksFn(ctx, clientID, lastTS, lastTaskID, limit)
	}
	return nil, nil
}
func (f *fakeCursorStore) LoadMessagePayload(ctx context.Context, messageID uuid.UUID) ([]byte, error) {
	if f.loadMessagePayloadFn != nil {
		return f.loadMessagePayloadFn(ctx, messageID)
	}
	return nil, nil
}

type fakeSharedCompletionStore struct {
	markCalls atomic.Int64
	taskID    uuid.UUID
	group     string
}

func (f *fakeSharedCompletionStore) EnsureSchema(context.Context) error { return nil }
func (f *fakeSharedCompletionStore) AppendShareGroupTask(context.Context, time.Time, *sharedsubscription.ShareGroupTask) error {
	panic("not used")
}
func (f *fakeSharedCompletionStore) ReadShareGroupTasks(context.Context, string, time.Time, uuid.UUID, int) ([]*sharedsubscription.ShareGroupTask, error) {
	panic("not used")
}
func (f *fakeSharedCompletionStore) MarkShareGroupTaskProcessed(_ context.Context, taskID uuid.UUID, group string) error {
	f.taskID = taskID
	f.group = group
	f.markCalls.Add(1)
	return nil
}
func (f *fakeSharedCompletionStore) AtomicUpdateTaskStatus(context.Context, uuid.UUID, string, sharedsubscription.TaskStatus, sharedsubscription.TaskStatus) (bool, error) {
	panic("not used")
}
func (f *fakeSharedCompletionStore) GetUnAckedSharedSubscriptionTasks(context.Context, string, string, time.Time, uuid.UUID) ([]*sharedsubscription.ShareGroupTask, error) {
	panic("not used")
}
func (f *fakeSharedCompletionStore) RollbackSharedSubscriptionTask(context.Context, *sharedsubscription.ShareGroupTask) error {
	panic("not used")
}
func (f *fakeSharedCompletionStore) QueryProcessingTasksBefore(context.Context, string, time.Time) ([]*sharedsubscription.ShareGroupTask, error) {
	panic("not used")
}
func (f *fakeSharedCompletionStore) CheckDeliveryTaskExists(context.Context, string, uuid.UUID) (bool, error) {
	panic("not used")
}
func (f *fakeSharedCompletionStore) ReadShareGroupCursor(context.Context, string) (*sharedsubscription.ShareGroupCursor, error) {
	panic("not used")
}
func (f *fakeSharedCompletionStore) AppendShareGroupCursor(context.Context, string, *sharedsubscription.ShareGroupCursor) error {
	panic("not used")
}
func (f *fakeSharedCompletionStore) QueryShareGroupTaskByMessageID(context.Context, string, uuid.UUID, []sharedsubscription.TaskStatus) (*sharedsubscription.ShareGroupTask, error) {
	panic("not used")
}

// Test: initial connection should read once then wait; a wake should trigger an immediate probe.
func TestDeliveryRunner_InitialProbeThenWakeTriggersProbe(t *testing.T) {
	// Minimal logger avoids depending on loading ../../../etc/config.yaml.
	// SkyLogger has unexported fields; only set the embedded zerolog.Logger.
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	// Configure runner to have long fallback to avoid timeout probe during test.
	_ = func() error { _, err := config.Load("../../../etc/config.yaml"); return err }()
	cfg := mustLoadConfigForTest(t)
	cfg.Delivery = config.DeliveryRunner{
		WakeMaxWait:                 5 * time.Second,
		WakeReadRetryMaxAttempts:    0,
		WakeReadRetryBackoffInitial: 10 * time.Millisecond,
		WakeReadRetryBackoffMax:     10 * time.Millisecond,
		InflightWaitTick:            50 * time.Millisecond,
	}

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
	cl.deliveryWakeCh = make(chan struct{}, 1)
	cl.component = &Component{
		deliveryCursorStore: delivery.CursorStore(cs),
	}

	go cl.runClientDeliveryRunner()

	// Initial probe should happen quickly (exact timing not guaranteed, so allow a small window).
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if cs.readTasksCalls.Load() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if cs.readTasksCalls.Load() != 1 {
		t.Fatalf("expected exactly 1 initial ReadTasks call, got %d", cs.readTasksCalls.Load())
	}

	// Wake should trigger a second probe quickly.
	cl.deliveryWakeCh <- struct{}{}
	deadline = time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if cs.readTasksCalls.Load() >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if cs.readTasksCalls.Load() < 2 {
		t.Fatalf("expected wake to trigger another ReadTasks call, got %d", cs.readTasksCalls.Load())
	}

	cl.cancel(nil)
	_ = c2.Close()
}

// Test: wake-triggered read error should retry a few times with backoff, but should stop once it succeeds (even if empty).
func TestDeliveryRunner_WakeReadErrorRetriesThenStopsOnSuccess(t *testing.T) {
	// Minimal logger avoids depending on loading ../../../etc/config.yaml.
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	_ = func() error { _, err := config.Load("../../../etc/config.yaml"); return err }()
	cfg := mustLoadConfigForTest(t)
	cfg.Delivery = config.DeliveryRunner{
		WakeMaxWait:                 5 * time.Second,
		WakeReadRetryMaxAttempts:    3,
		WakeReadRetryBackoffInitial: 5 * time.Millisecond,
		WakeReadRetryBackoffMax:     5 * time.Millisecond,
		InflightWaitTick:            50 * time.Millisecond,
	}

	var failCount atomic.Int64
	cs := &fakeCursorStore{}
	cs.readTasksFn = func(ctx context.Context, clientID string, lastTS time.Time, lastTaskID uuid.UUID, limit int) ([]*store.DeliveryTask, error) {
		// First call (initial probe) empty success.
		// Second call (wake probe) error.
		// Then retries: error twice, then success empty => stop.
		call := cs.readTasksCalls.Load()
		if call == 2 {
			return nil, errors.New("boom")
		}
		if call >= 3 && call <= 4 {
			failCount.Add(1)
			return nil, errors.New("still failing")
		}
		return nil, nil
	}

	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	cl := NewClient(c1)
	cl.ID = "test-client"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	cl.deliveryWakeCh = make(chan struct{}, 1)
	cl.component = &Component{
		deliveryCursorStore: delivery.CursorStore(cs),
	}
	go cl.runClientDeliveryRunner()

	// Wait for initial probe.
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if cs.readTasksCalls.Load() >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Trigger wake to start error+retry path.
	cl.deliveryWakeCh <- struct{}{}

	// Expect a few calls total: initial(1) + wake(1) + retries(up to 3).
	deadline = time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if cs.readTasksCalls.Load() >= 4 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if cs.readTasksCalls.Load() < 4 {
		t.Fatalf("expected retries after wake read error, got calls=%d", cs.readTasksCalls.Load())
	}

	// Ensure it doesn't spin: after success (empty), it should go back to waiting, not immediately retry again.
	callsAfter := cs.readTasksCalls.Load()
	time.Sleep(50 * time.Millisecond)
	if cs.readTasksCalls.Load() > callsAfter+1 {
		t.Fatalf("expected runner to stop retrying once ReadTasks succeeds; calls grew from %d to %d", callsAfter, cs.readTasksCalls.Load())
	}

	cl.cancel(nil)
	_ = c2.Close()
}

func TestDeliverSingleClientDeliveryTask_RAPFalseClearsRetain(t *testing.T) {
	// Minimal logger avoids depending on loading ../../../etc/config.yaml.
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	cl := NewClient(c1)
	cl.ID = "c1"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	// Ensure plugin manager is non-nil for the Client.write path.
	cl.component.plugin = &plugin.Plugins{}

	// Build message payload with retain=true, QoS0.
	p := &packets.Publish{Topic: "t/a", QoS: 0, Retain: true, Payload: []byte("x")}
	m := &brokerpublish.Message{SendClientID: "publisher"}
	m.SetPublish(p)
	raw, err := serializer.Serializer.Encode(m)
	if err != nil {
		t.Fatalf("encode message: %v", err)
	}

	cs := &fakeCursorStore{
		loadMessagePayloadFn: func(ctx context.Context, messageID uuid.UUID) ([]byte, error) { return raw, nil },
	}
	lastTS := time.Unix(0, 0)
	lastTask := uuid.Nil

	task := &store.DeliveryTask{
		TS:                time.Unix(1, 0),
		TaskID:            uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		ClientID:          "c1",
		MessageID:         uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		DeliveryQoS:       0,
		RetainAsPublished: false,
	}
	// Use concurrent read/write for net.Pipe(): WriteTo blocks until the peer reads.
	errCh := make(chan error, 1)
	go func() {
		errCh <- cl.deliverSingleClientDeliveryTask(delivery.CursorStore(cs), task, &lastTS, &lastTask)
	}()

	_ = c2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	got, err := wire.Decode(c2, wire.DecodeOptions{})
	if err != nil {
		t.Fatalf("read packet: %v", err)
	}
	pub, ok := got.Content.(*packets.Publish)
	if !ok || pub == nil {
		t.Fatalf("expected publish, got %T", got.Content)
	}
	if pub.Retain {
		t.Fatalf("expected retain cleared when RAP=false")
	}
	if derr := <-errCh; derr != nil {
		t.Fatalf("deliver: %v", derr)
	}
}

func TestDeliverSingleClientDeliveryTaskRecordsInitialDeliveryMetrics(t *testing.T) {
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	cl := NewClient(c1)
	cl.ID = "c1"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	cl.component.plugin = &plugin.Plugins{}

	p := &packets.Publish{Topic: "t/a", QoS: 0, Payload: []byte("x")}
	m := &brokerpublish.Message{SendClientID: "publisher"}
	m.SetPublish(p)
	raw, err := serializer.Serializer.Encode(m)
	if err != nil {
		t.Fatalf("encode message: %v", err)
	}

	cs := &fakeCursorStore{
		loadMessagePayloadFn: func(ctx context.Context, messageID uuid.UUID) ([]byte, error) { return raw, nil },
	}
	lastTS := time.Unix(0, 0)
	lastTask := uuid.Nil
	task := &store.DeliveryTask{
		TS:                time.Now().Add(-time.Second),
		TaskID:            uuid.New(),
		ClientID:          "c1",
		MessageID:         uuid.New(),
		DeliveryQoS:       1,
		RetainAsPublished: true,
	}
	delayBefore := clientHistogramSampleCount(t, metric.DeliveryFirstSendDelaySeconds, map[string]string{
		"path": "normal",
		"qos":  "1",
	})
	attemptBefore := testutil.ToFloat64(metric.DeliverySendAttemptTotal.WithLabelValues("normal", "1", "initial"))

	errCh := make(chan error, 1)
	go func() {
		errCh <- cl.deliverSingleClientDeliveryTask(delivery.CursorStore(cs), task, &lastTS, &lastTask)
	}()

	_ = c2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, err := wire.Decode(c2, wire.DecodeOptions{}); err != nil {
		t.Fatalf("read packet: %v", err)
	}
	if derr := <-errCh; derr != nil {
		t.Fatalf("deliver: %v", derr)
	}

	assertClientHistogramSampleCount(t, metric.DeliveryFirstSendDelaySeconds, map[string]string{
		"path": "normal",
		"qos":  "1",
	}, delayBefore+1)
	if got := testutil.ToFloat64(metric.DeliverySendAttemptTotal.WithLabelValues("normal", "1", "initial")); got != attemptBefore+1 {
		t.Fatalf("initial send attempt counter = %v, want %v", got, attemptBefore+1)
	}
}

func TestMaybeRetransmitOutgoingInflightRecordsRetransmitMetric(t *testing.T) {
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	cl := NewClient(c1)
	cl.ID = "c1"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	cl.component.plugin = &plugin.Plugins{}

	messageID := uuid.New()
	task := &store.DeliveryTask{
		TS:                time.Now().Add(-time.Second),
		TaskID:            uuid.New(),
		ClientID:          "c1",
		MessageID:         messageID,
		DeliveryQoS:       1,
		RetainAsPublished: true,
	}
	p := &packets.Publish{Topic: "t/a", QoS: 0, Payload: []byte("x")}
	m := &brokerpublish.Message{SendClientID: "publisher"}
	m.SetPublish(p)
	raw, err := serializer.Serializer.Encode(m)
	if err != nil {
		t.Fatalf("encode message: %v", err)
	}
	cs := &fakeCursorStore{
		readTasksFn: func(ctx context.Context, clientID string, lastTS time.Time, lastTaskID uuid.UUID, limit int) ([]*store.DeliveryTask, error) {
			return []*store.DeliveryTask{task}, nil
		},
		loadMessagePayloadFn: func(ctx context.Context, messageID uuid.UUID) ([]byte, error) { return raw, nil },
	}
	cl.component.deliveryCursorStore = delivery.CursorStore(cs)
	cl.outgoingInflight.Put(&outgoingInflightEntry{
		PacketID:     10,
		QoS:          1,
		TaskTS:       task.TS,
		TaskID:       task.TaskID,
		MessageID:    messageID,
		FirstPubTime: time.Now().Add(-time.Second),
		LastSendTime: time.Now().Add(-time.Second),
		State:        outInflightWaitingPubAck,
	})
	before := testutil.ToFloat64(metric.DeliverySendAttemptTotal.WithLabelValues("normal", "1", "retransmit"))

	done := make(chan struct{})
	go func() {
		cl.maybeRetransmitOutgoingInflight(config.DeliveryRunner{
			InflightRetransmitInterval: time.Nanosecond,
			InflightMaxAge:             time.Hour,
			InflightMaxRetries:         5,
		})
		close(done)
	}()

	_ = c2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, err := wire.Decode(c2, wire.DecodeOptions{}); err != nil {
		t.Fatalf("read retransmit packet: %v", err)
	}
	<-done
	if got := testutil.ToFloat64(metric.DeliverySendAttemptTotal.WithLabelValues("normal", "1", "retransmit")); got != before+1 {
		t.Fatalf("retransmit send attempt counter = %v, want %v", got, before+1)
	}
}

func TestDeliverSingleClientDeliveryTask_SubscriptionIdentifierFromQueue(t *testing.T) {
	// Minimal logger avoids depending on loading ../../../etc/config.yaml.
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	cl := NewClient(c1)
	cl.ID = "c1"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	// Ensure plugin manager is non-nil for the Client.write path.
	cl.component.plugin = &plugin.Plugins{}

	p := &packets.Publish{Topic: "t/a", QoS: 0, Retain: false, Payload: []byte("x")}
	m := &brokerpublish.Message{SendClientID: "publisher"}
	m.SetPublish(p)
	raw, err := serializer.Serializer.Encode(m)
	if err != nil {
		t.Fatalf("encode message: %v", err)
	}

	cs := &fakeCursorStore{
		loadMessagePayloadFn: func(ctx context.Context, messageID uuid.UUID) ([]byte, error) { return raw, nil },
	}
	lastTS := time.Unix(0, 0)
	lastTask := uuid.Nil

	task := &store.DeliveryTask{
		TS:                time.Unix(1, 0),
		TaskID:            uuid.MustParse("00000000-0000-0000-0000-000000000011"),
		ClientID:          "c1",
		MessageID:         uuid.MustParse("00000000-0000-0000-0000-000000000012"),
		DeliveryQoS:       0,
		SubscriptionIDs:   []int32{2, 1},
		RetainAsPublished: true,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- cl.deliverSingleClientDeliveryTask(delivery.CursorStore(cs), task, &lastTS, &lastTask)
	}()

	_ = c2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	got, err := wire.Decode(c2, wire.DecodeOptions{})
	if err != nil {
		t.Fatalf("read packet: %v", err)
	}
	pub, ok := got.Content.(*packets.Publish)
	if !ok || pub == nil {
		t.Fatalf("expected publish, got %T", got.Content)
	}
	if pub.Properties == nil || len(pub.Properties.SubscriptionIdentifier) == 0 {
		t.Fatalf("expected SubscriptionIdentifier set")
	}
	// SubscriptionIDs is [2,1], so we should have both values.
	if len(pub.Properties.SubscriptionIdentifier) != 2 {
		t.Fatalf("expected 2 SubscriptionIdentifiers, got %d: %v", len(pub.Properties.SubscriptionIdentifier), pub.Properties.SubscriptionIdentifier)
	}
	// Order should be [2, 1] as in JSON
	if pub.Properties.SubscriptionIdentifier[0] != 2 || pub.Properties.SubscriptionIdentifier[1] != 1 {
		t.Fatalf("expected SubscriptionIdentifier=[2,1], got %v", pub.Properties.SubscriptionIdentifier)
	}
	if derr := <-errCh; derr != nil {
		t.Fatalf("deliver: %v", derr)
	}
}

func TestDeliverSingleClientDeliveryTask_FiltersInvalidSubscriptionIdentifierFromQueue(t *testing.T) {
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	cl := NewClient(c1)
	cl.ID = "c1"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	cl.component.plugin = &plugin.Plugins{}

	p := &packets.Publish{Topic: "t/a", QoS: 0, Retain: false, Payload: []byte("x")}
	m := &brokerpublish.Message{SendClientID: "publisher"}
	m.SetPublish(p)
	raw, err := serializer.Serializer.Encode(m)
	if err != nil {
		t.Fatalf("encode message: %v", err)
	}

	cs := &fakeCursorStore{
		loadMessagePayloadFn: func(ctx context.Context, messageID uuid.UUID) ([]byte, error) { return raw, nil },
	}
	lastTS := time.Unix(0, 0)
	lastTask := uuid.Nil

	task := &store.DeliveryTask{
		TS:                time.Unix(1, 0),
		TaskID:            uuid.MustParse("00000000-0000-0000-0000-000000000111"),
		ClientID:          "c1",
		MessageID:         uuid.MustParse("00000000-0000-0000-0000-000000000112"),
		DeliveryQoS:       0,
		SubscriptionIDs:   []int32{-1, 0, 1, 268435455, 268435456},
		RetainAsPublished: true,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- cl.deliverSingleClientDeliveryTask(delivery.CursorStore(cs), task, &lastTS, &lastTask)
	}()

	_ = c2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	got, err := wire.Decode(c2, wire.DecodeOptions{})
	if err != nil {
		t.Fatalf("read packet: %v", err)
	}
	pub, ok := got.Content.(*packets.Publish)
	if !ok || pub == nil {
		t.Fatalf("expected publish, got %T", got.Content)
	}
	if pub.Properties == nil {
		t.Fatal("expected publish properties")
	}
	if len(pub.Properties.SubscriptionIdentifier) != 2 {
		t.Fatalf("expected 2 valid subscription identifiers, got %v", pub.Properties.SubscriptionIdentifier)
	}
	if pub.Properties.SubscriptionIdentifier[0] != 1 || pub.Properties.SubscriptionIdentifier[1] != 268435455 {
		t.Fatalf("unexpected subscription identifiers: %v", pub.Properties.SubscriptionIdentifier)
	}
	if derr := <-errCh; derr != nil {
		t.Fatalf("deliver: %v", derr)
	}
}

func TestDeliverSingleClientDeliveryTask_CompletesSharedTaskAfterQoS0Delivery(t *testing.T) {
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}

	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	sharedStore := &fakeSharedCompletionStore{}
	cl := NewClient(c1)
	cl.ID = "c1"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	cl.component.plugin = &plugin.Plugins{}
	cl.component.sharedSubscriptionManager = shared_manager.NewSharedSubscriptionManager(
		sharedStore,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		0,
		nil,
		nil,
	)

	p := &packets.Publish{Topic: "t/a", QoS: 0, Retain: false, Payload: []byte("x")}
	m := &brokerpublish.Message{SendClientID: "publisher"}
	m.SetPublish(p)
	raw, err := serializer.Serializer.Encode(m)
	if err != nil {
		t.Fatalf("encode message: %v", err)
	}

	cs := &fakeCursorStore{
		loadMessagePayloadFn: func(ctx context.Context, messageID uuid.UUID) ([]byte, error) { return raw, nil },
	}
	lastTS := time.Unix(0, 0)
	lastTask := uuid.Nil
	sharedTaskID := uuid.MustParse("00000000-0000-0000-0000-000000000033")

	task := &store.DeliveryTask{
		TS:                time.Unix(1, 0),
		TaskID:            uuid.MustParse("00000000-0000-0000-0000-000000000031"),
		ClientID:          "c1",
		MessageID:         uuid.MustParse("00000000-0000-0000-0000-000000000032"),
		DeliveryQoS:       0,
		RetainAsPublished: true,
		ShareGroup:        "g",
		SharedTaskID:      sharedTaskID,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- cl.deliverSingleClientDeliveryTask(delivery.CursorStore(cs), task, &lastTS, &lastTask)
	}()

	_ = c2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, err := wire.Decode(c2, wire.DecodeOptions{}); err != nil {
		t.Fatalf("read packet: %v", err)
	}
	if derr := <-errCh; derr != nil {
		t.Fatalf("deliver: %v", derr)
	}
	if sharedStore.markCalls.Load() != 1 {
		t.Fatalf("expected shared task completed once, got %d", sharedStore.markCalls.Load())
	}
	if sharedStore.group != "g" || sharedStore.taskID != sharedTaskID {
		t.Fatalf("expected completed shared task g/%s, got %s/%s", sharedTaskID, sharedStore.group, sharedStore.taskID)
	}
}

func TestDeliverSingleClientDeliveryTask_NoLocalSkipsAndAdvancesCursor(t *testing.T) {
	// Minimal logger avoids depending on loading ../../../etc/config.yaml.
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	cl := NewClient(c1)
	cl.ID = "c1"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	// Ensure plugin manager is non-nil for the Client.write path.
	cl.component.plugin = &plugin.Plugins{}

	p := &packets.Publish{Topic: "t/a", QoS: 0, Retain: false, Payload: []byte("x")}
	// Publisher is the same as receiver (NL should drop).
	m := &brokerpublish.Message{SendClientID: "c1"}
	m.SetPublish(p)
	raw, err := serializer.Serializer.Encode(m)
	if err != nil {
		t.Fatalf("encode message: %v", err)
	}

	var appended atomic.Int64
	cs := &fakeCursorStore{
		loadMessagePayloadFn: func(ctx context.Context, messageID uuid.UUID) ([]byte, error) { return raw, nil },
		advanceCursorFn: func(ctx context.Context, cursor store.DeliveryCursor) error {
			if cursor.ClientID != "c1" {
				t.Fatalf("expected AdvanceCursor clientID=c1, got %q", cursor.ClientID)
			}
			appended.Add(1)
			return nil
		},
	}
	lastTS := time.Unix(0, 0)
	lastTask := uuid.Nil
	taskTS := time.Unix(3, 0)
	taskID := uuid.MustParse("00000000-0000-0000-0000-000000000021")

	task := &store.DeliveryTask{
		TS:                taskTS,
		TaskID:            taskID,
		ClientID:          "c1",
		MessageID:         uuid.MustParse("00000000-0000-0000-0000-000000000022"),
		DeliveryQoS:       0,
		NoLocal:           true,
		RetainAsPublished: true,
	}
	if err := cl.deliverSingleClientDeliveryTask(delivery.CursorStore(cs), task, &lastTS, &lastTask); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if appended.Load() != 1 {
		t.Fatalf("expected AdvanceCursor called once, got %d", appended.Load())
	}
	if !lastTS.Equal(taskTS) || lastTask != taskID {
		t.Fatalf("expected last cursor updated to task, got ts=%v id=%v", lastTS, lastTask)
	}

	_ = c2.SetReadDeadline(time.Now().Add(20 * time.Millisecond))
	_, rerr := wire.Decode(c2, wire.DecodeOptions{})
	if rerr == nil {
		t.Fatalf("expected no packet delivered for NL, but read succeeded")
	}
}

func TestDeliverSingleClientDeliveryTask_MissingPayloadAdvancesCursor(t *testing.T) {
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	c1, c2 := net.Pipe()
	defer func() { _ = c1.Close() }()
	defer func() { _ = c2.Close() }()

	cl := NewClient(c1)
	cl.ID = "c1"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	cl.component.plugin = &plugin.Plugins{}

	var advanced atomic.Int64
	cs := &fakeCursorStore{
		loadMessagePayloadFn: func(ctx context.Context, messageID uuid.UUID) ([]byte, error) {
			return nil, store.ErrMessagePayloadNotFound
		},
		advanceCursorFn: func(ctx context.Context, cursor store.DeliveryCursor) error {
			if cursor.ClientID != "c1" {
				t.Fatalf("expected AdvanceCursor clientID=c1, got %q", cursor.ClientID)
			}
			advanced.Add(1)
			return nil
		},
	}
	lastTS := time.Unix(0, 0)
	lastTask := uuid.Nil
	taskTS := time.Unix(4, 0)
	taskID := uuid.MustParse("00000000-0000-0000-0000-000000000041")

	task := &store.DeliveryTask{
		TS:                taskTS,
		TaskID:            taskID,
		ClientID:          "c1",
		MessageID:         uuid.MustParse("00000000-0000-0000-0000-000000000042"),
		DeliveryQoS:       0,
		RetainAsPublished: true,
	}
	if err := cl.deliverSingleClientDeliveryTask(delivery.CursorStore(cs), task, &lastTS, &lastTask); err != nil {
		t.Fatalf("deliver should drop missing payload without error: %v", err)
	}
	if advanced.Load() != 1 {
		t.Fatalf("expected AdvanceCursor called once, got %d", advanced.Load())
	}
	if !lastTS.Equal(taskTS) || lastTask != taskID {
		t.Fatalf("expected last cursor updated to task, got ts=%v id=%v", lastTS, lastTask)
	}

	_ = c2.SetReadDeadline(time.Now().Add(20 * time.Millisecond))
	_, rerr := wire.Decode(c2, wire.DecodeOptions{})
	if rerr == nil {
		t.Fatalf("expected no packet delivered for missing payload, but read succeeded")
	}
}

func TestDeliverSingleClientDeliveryTask_RegistersInflightBeforeWrite(t *testing.T) {
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}

	var advanced atomic.Int64
	var cl *Client
	conn := &callbackConn{}
	cl = NewClient(conn)
	cl.ID = "c1"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	cl.component.plugin = &plugin.Plugins{}
	cl.publishBucket = rate.NewBucket(1)
	cl.deliveryWakeCh = make(chan struct{}, 1)

	conn.onFirstWrite = func() {
		if cl.outgoingInflight.Len() != 1 {
			t.Fatalf("expected inflight to be registered before socket write")
		}
		if err := NewClientHandler(cl).handlePubAck(context.Background(), &packets.Puback{PacketID: 1}); err != nil {
			t.Fatalf("handle puback during write: %v", err)
		}
	}

	p := &packets.Publish{Topic: "t/a", QoS: 0, Payload: []byte("x")}
	m := &brokerpublish.Message{SendClientID: "publisher"}
	m.SetPublish(p)
	raw, err := serializer.Serializer.Encode(m)
	if err != nil {
		t.Fatalf("encode message: %v", err)
	}

	cs := &fakeCursorStore{
		loadMessagePayloadFn: func(ctx context.Context, messageID uuid.UUID) ([]byte, error) { return raw, nil },
		advanceCursorFn: func(ctx context.Context, cursor store.DeliveryCursor) error {
			advanced.Add(1)
			return nil
		},
	}
	cl.component.deliveryCursorStore = delivery.CursorStore(cs)

	packetIDs := NewPacketIDFactory()
	packetIDs.SetID(0)
	cl.packetIDFactory = packetIDs
	lastTS := time.Unix(0, 0)
	lastTask := uuid.Nil
	task := &store.DeliveryTask{
		TS:                time.Unix(10, 0),
		TaskID:            uuid.MustParse("00000000-0000-0000-0000-000000000051"),
		ClientID:          "c1",
		MessageID:         uuid.MustParse("00000000-0000-0000-0000-000000000052"),
		DeliveryQoS:       1,
		RetainAsPublished: true,
	}

	if err := cl.deliverSingleClientDeliveryTask(delivery.CursorStore(cs), task, &lastTS, &lastTask); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if got := advanced.Load(); got != 1 {
		t.Fatalf("expected PUBACK during write to advance cursor once, got %d", got)
	}
	if got := cl.outgoingInflight.Len(); got != 0 {
		t.Fatalf("expected inflight cleared by PUBACK, got %d", got)
	}
	if got := cl.publishBucket.RemainingToken(); got != 1 {
		t.Fatalf("expected flow token released after matching PUBACK, got %d", got)
	}
}

func TestDeliverClientDeliveryTasksUsesReceiveMaximumWindow(t *testing.T) {
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}

	conn := &bufferConn{}
	cl := NewClient(conn)
	cl.ID = "c1"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	cl.component.plugin = &plugin.Plugins{}
	cl.publishBucket = rate.NewBucket(2)

	payloads := make(map[uuid.UUID][]byte)
	tasks := make([]*store.DeliveryTask, 0, 3)
	for i := 0; i < 3; i++ {
		messageID := uuid.MustParse(fmt.Sprintf("00000000-0000-0000-0000-00000000007%d", i+1))
		taskID := uuid.MustParse(fmt.Sprintf("00000000-0000-0000-0000-00000000008%d", i+1))
		p := &packets.Publish{Topic: fmt.Sprintf("jobs/%d", i+1), QoS: 0, Payload: []byte("x")}
		m := &brokerpublish.Message{SendClientID: "publisher"}
		m.SetPublish(p)
		raw, err := serializer.Serializer.Encode(m)
		if err != nil {
			t.Fatalf("encode message %d: %v", i, err)
		}
		payloads[messageID] = raw
		tasks = append(tasks, &store.DeliveryTask{
			TS:                time.Unix(int64(20+i), 0),
			TaskID:            taskID,
			ClientID:          "c1",
			MessageID:         messageID,
			DeliveryQoS:       1,
			RetainAsPublished: true,
		})
	}

	cs := &fakeCursorStore{
		loadMessagePayloadFn: func(_ context.Context, messageID uuid.UUID) ([]byte, error) {
			return payloads[messageID], nil
		},
	}
	lastTS := time.Unix(0, 0)
	lastTask := uuid.Nil

	cl.deliverClientDeliveryTasks(delivery.CursorStore(cs), tasks, time.Millisecond, &lastTS, &lastTask)

	if got := cl.outgoingInflight.Len(); got != 2 {
		t.Fatalf("expected two downlink inflight messages for ReceiveMaximum=2, got %d", got)
	}
	if got := cl.publishBucket.RemainingToken(); got != 0 {
		t.Fatalf("expected receive maximum window to be full, remaining tokens=%d", got)
	}
	if !lastTS.Equal(tasks[1].TS) || lastTask != tasks[1].TaskID {
		t.Fatalf("expected in-memory cursor at second task, got ts=%v task=%s", lastTS, lastTask)
	}

	gotPackets := 0
	readBuf := bytes.NewBuffer(conn.Bytes())
	for {
		cp, err := wire.Decode(readBuf, wire.DecodeOptions{})
		if err != nil {
			break
		}
		if cp.FixedHeader.Type != packets.PUBLISH {
			t.Fatalf("expected PUBLISH, got 0x%X", cp.FixedHeader.Type)
		}
		gotPackets++
	}
	if gotPackets != 2 {
		t.Fatalf("expected two PUBLISH packets written, got %d", gotPackets)
	}
}

func TestHandlePubAckAdvancesCursorInTaskOrderWhenAcksArriveOutOfOrder(t *testing.T) {
	_ = func() error { _, err := config.Load("../../../etc/config.yaml"); return err }()
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	c := NewClient(&bufferConn{})
	c.ID = "client-a"
	c.ctx = context.Background()
	c.publishBucket = rate.NewBucket(2)
	c.publishBucket.GetToken(c.ctx)
	c.publishBucket.GetToken(c.ctx)

	var advanced []store.DeliveryCursor
	c.component.deliveryCursorStore = delivery.CursorStore(&fakeCursorStore{
		advanceCursorFn: func(_ context.Context, cursor store.DeliveryCursor) error {
			advanced = append(advanced, cursor)
			return nil
		},
	})

	firstTaskID := uuid.MustParse("00000000-0000-0000-0000-000000000091")
	secondTaskID := uuid.MustParse("00000000-0000-0000-0000-000000000092")
	c.outgoingInflight.Put(&outgoingInflightEntry{
		PacketID:      1,
		QoS:           1,
		TaskTS:        time.Unix(30, 0),
		TaskID:        firstTaskID,
		State:         outInflightWaitingPubAck,
		FlowTokenHeld: true,
	})
	c.outgoingInflight.Put(&outgoingInflightEntry{
		PacketID:      2,
		QoS:           1,
		TaskTS:        time.Unix(31, 0),
		TaskID:        secondTaskID,
		State:         outInflightWaitingPubAck,
		FlowTokenHeld: true,
	})

	if err := NewClientHandler(c).handlePubAck(context.Background(), &packets.Puback{PacketID: 2}); err != nil {
		t.Fatalf("handle second puback: %v", err)
	}
	if len(advanced) != 0 {
		t.Fatalf("out-of-order second ACK must not advance cursor before first ACK, got %+v", advanced)
	}
	if got := c.publishBucket.RemainingToken(); got != 1 {
		t.Fatalf("second ACK should release its receive maximum token, remaining=%d", got)
	}
	if e, ok := c.outgoingInflight.Get(2); !ok || !e.Acked {
		t.Fatalf("second ACK should keep an acked entry behind first inflight, entry=%+v ok=%v", e, ok)
	}

	if err := NewClientHandler(c).handlePubAck(context.Background(), &packets.Puback{PacketID: 1}); err != nil {
		t.Fatalf("handle first puback: %v", err)
	}
	if len(advanced) != 2 {
		t.Fatalf("first ACK should advance both contiguous acked tasks, got %+v", advanced)
	}
	if advanced[0].LastTaskID != firstTaskID || advanced[1].LastTaskID != secondTaskID {
		t.Fatalf("expected cursor order first->second, got %+v", advanced)
	}
	if got := c.outgoingInflight.Len(); got != 0 {
		t.Fatalf("expected all acked inflight entries removed, got %d", got)
	}
	if got := c.publishBucket.RemainingToken(); got != 2 {
		t.Fatalf("expected all receive maximum tokens released, got %d", got)
	}
}

func assertClientHistogramSampleCount(t *testing.T, collector prometheus.Collector, labels map[string]string, want uint64) {
	t.Helper()
	if got := clientHistogramSampleCount(t, collector, labels); got != want {
		t.Fatalf("histogram sample count for labels %v = %d, want %d", labels, got, want)
	}
}

func clientHistogramSampleCount(t *testing.T, collector prometheus.Collector, labels map[string]string) uint64 {
	t.Helper()
	metrics := make(chan prometheus.Metric, 16)
	go func() {
		collector.Collect(metrics)
		close(metrics)
	}()
	for item := range metrics {
		dtoMetric := &dto.Metric{}
		if err := item.Write(dtoMetric); err != nil {
			t.Fatalf("write metric: %v", err)
		}
		if !clientMetricLabelsMatch(dtoMetric, labels) {
			continue
		}
		if dtoMetric.Histogram == nil {
			return 0
		}
		return dtoMetric.Histogram.GetSampleCount()
	}
	return 0
}

func clientMetricLabelsMatch(item *dto.Metric, labels map[string]string) bool {
	if item == nil {
		return false
	}
	got := make(map[string]string, len(item.Label))
	for _, pair := range item.Label {
		got[pair.GetName()] = pair.GetValue()
	}
	for name, wantValue := range labels {
		if got[name] != wantValue {
			return false
		}
	}
	return true
}
