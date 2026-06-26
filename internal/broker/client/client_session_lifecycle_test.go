package client

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/config"
	shared_manager "github.com/BAN1ce/skyTree/internal/broker/sharedsubscription/manager"
	"github.com/BAN1ce/skyTree/internal/broker/willdelay"
	"github.com/BAN1ce/skyTree/logger"
	brokerstore "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/subscription"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/wire"
	"github.com/BAN1ce/skyTree/proto/proto_topic"
	"github.com/BAN1ce/skyTree/proto/proto_will_delay"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
)

type recordingSubCenter struct {
	deleteCount atomic.Int64
	lastClient  string
	lastToken   string
}

var _ subscription.Center = (*recordingSubCenter)(nil)

func (s *recordingSubCenter) CreateSub(context.Context, *proto_topic.SubRequest) (*proto_topic.SubResponse, error) {
	return &proto_topic.SubResponse{}, nil
}

func (s *recordingSubCenter) DeleteSub(context.Context, *proto_topic.UnSubRequest) (*proto_topic.UnSubResponse, error) {
	return &proto_topic.UnSubResponse{}, nil
}

func (s *recordingSubCenter) GetAllMatchTopics(context.Context, *proto_topic.GetAllMatchTopicsRequest) (*proto_topic.GetAllMatchTopicsResponse, error) {
	return &proto_topic.GetAllMatchTopicsResponse{}, nil
}

func (s *recordingSubCenter) GetAllMatchTopicsForWildTopic(context.Context, *proto_topic.GetAllMatchTopicsForWildTopicRequest) (*proto_topic.GetAllMatchTopicsForWildTopicResponse, error) {
	return &proto_topic.GetAllMatchTopicsForWildTopicResponse{}, nil
}

func (s *recordingSubCenter) DeleteClient(ctx context.Context, req *proto_topic.DeleteClientRequest) (*proto_topic.DeleteClientResponse, error) {
	s.deleteCount.Add(1)
	s.lastClient = req.GetClientID()
	s.lastToken = req.GetOwnerToken()
	return &proto_topic.DeleteClientResponse{Success: true}, nil
}

func (s *recordingSubCenter) DeleteTopic(context.Context, *proto_topic.DeleteTopicRequest) (*proto_topic.DeleteTopicResponse, error) {
	return &proto_topic.DeleteTopicResponse{}, nil
}

func (s *recordingSubCenter) GetAllMatchClient(context.Context, *proto_topic.GetAllSubTopicClientRequest) (*proto_topic.GetAllSubTopicClientResponse, error) {
	return &proto_topic.GetAllSubTopicClientResponse{}, nil
}

func (s *recordingSubCenter) GetAllMatchClientV2(context.Context, *proto_topic.GetAllMatchClientV2Request) (*proto_topic.GetAllMatchClientV2Response, error) {
	return &proto_topic.GetAllMatchClientV2Response{}, nil
}

func (s *recordingSubCenter) GetSubTree(context.Context, *proto_topic.GetSubTreeRequest) (*proto_topic.GetSubTreeResponse, error) {
	return &proto_topic.GetSubTreeResponse{}, nil
}

func (s *recordingSubCenter) SetClientOwnerToken(context.Context, *proto_topic.SetClientOwnerTokenRequest) (*proto_topic.SetClientOwnerTokenResponse, error) {
	return &proto_topic.SetClientOwnerTokenResponse{Success: true}, nil
}

func (s *recordingSubCenter) GetClientSubscriptions(context.Context, *proto_topic.GetClientSubscriptionsRequest) (*proto_topic.GetClientSubscriptionsResponse, error) {
	return &proto_topic.GetClientSubscriptionsResponse{}, nil
}

func (s *recordingSubCenter) GetShareGroupMembers(context.Context, *proto_topic.GetShareGroupMembersRequest) (*proto_topic.GetShareGroupMembersResponse, error) {
	return &proto_topic.GetShareGroupMembersResponse{}, nil
}

type closeErrorConn struct {
	bufferConn
	err error
}

func (c *closeErrorConn) Close() error {
	return c.err
}

type recordingWillDelayCenter struct {
	addCount atomic.Int64
	addErr   error
	lastTask atomic.Value // stores *proto_will_delay.WillDelayTask
}

var _ willdelay.Center = (*recordingWillDelayCenter)(nil)

func (c *recordingWillDelayCenter) AddTask(_ context.Context, task *proto_will_delay.WillDelayTask) error {
	c.addCount.Add(1)
	if task != nil {
		if cloned, ok := proto.Clone(task).(*proto_will_delay.WillDelayTask); ok {
			c.lastTask.Store(cloned)
		} else {
			c.lastTask.Store(task)
		}
	}
	if c.addErr != nil {
		return c.addErr
	}
	return nil
}

func (c *recordingWillDelayCenter) LastTask() *proto_will_delay.WillDelayTask {
	raw := c.lastTask.Load()
	if raw == nil {
		return nil
	}
	task, _ := raw.(*proto_will_delay.WillDelayTask)
	return task
}

func (c *recordingWillDelayCenter) DeleteTask(context.Context, string) error {
	return nil
}

func (c *recordingWillDelayCenter) GetDueTasks(context.Context, int64) ([]*proto_will_delay.WillDelayTask, error) {
	return nil, nil
}

type recordingDeliveryCursorStore struct {
	deleteCount atomic.Int64
	lastClient  string
}

func (s *recordingDeliveryCursorStore) ReadCursor(context.Context, string) (*brokerstore.DeliveryCursor, error) {
	return nil, nil
}

func (s *recordingDeliveryCursorStore) AdvanceCursor(context.Context, brokerstore.DeliveryCursor) error {
	return nil
}

func (s *recordingDeliveryCursorStore) ReadTasks(context.Context, string, time.Time, uuid.UUID, int) ([]*brokerstore.DeliveryTask, error) {
	return nil, nil
}

func (s *recordingDeliveryCursorStore) LoadMessagePayload(context.Context, uuid.UUID) ([]byte, error) {
	return nil, brokerstore.ErrMessagePayloadNotFound
}

func (s *recordingDeliveryCursorStore) DeleteClientState(_ context.Context, clientID string) error {
	s.deleteCount.Add(1)
	s.lastClient = clientID
	return nil
}

func TestCloseWithSessionExpiryZeroCleansSubscriptionsAndPublishesDelayedWillImmediately(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	willCh := make(chan *brokerpublish.Message, 1)
	subCenter := &recordingSubCenter{}
	willDelayCenter := &recordingWillDelayCenter{}
	deliveryCursorStore := &recordingDeliveryCursorStore{}

	c := NewClient(
		&bufferConn{},
		WithSubCenter(subCenter),
		WithSessionCenter(&fakeSessionCenter{}),
		WithWillDelayCenter(willDelayCenter),
		WithNotifyWillMessageChan(willCh),
		WithDeliveryCursorStore(deliveryCursorStore),
	)
	c.ID = "client-a"
	c.ownerToken = "owner-token"
	c.ctx = context.Background()
	c.sessionExpiryInterval = 0
	c.connAckAccepted.Store(true)
	c.willMessage = &brokerpublish.Message{
		SendClientID: c.getID(),
		WillDelay:    time.Hour,
		Publish: &packets.Publish{
			Topic:   "will/topic",
			Payload: []byte("bye"),
			QoS:     1,
		},
	}

	if err := c.close(); err != nil {
		t.Fatalf("close client: %v", err)
	}

	if got := subCenter.deleteCount.Load(); got != 1 {
		t.Fatalf("expected subscriptions to be deleted once, got %d", got)
	}
	if subCenter.lastClient != "client-a" {
		t.Fatalf("expected delete client-a subscriptions, got %q", subCenter.lastClient)
	}
	if subCenter.lastToken != "owner-token" {
		t.Fatalf("expected delete to be fenced by owner token, got %q", subCenter.lastToken)
	}
	if got := deliveryCursorStore.deleteCount.Load(); got != 1 {
		t.Fatalf("expected delivery state to be deleted once, got %d", got)
	}
	if deliveryCursorStore.lastClient != "client-a" {
		t.Fatalf("expected delivery state delete for client-a, got %q", deliveryCursorStore.lastClient)
	}
	if got := willDelayCenter.addCount.Load(); got != 0 {
		t.Fatalf("expected no will-delay task when session expires immediately, got %d", got)
	}

	select {
	case msg := <-willCh:
		if msg.GetPublish().Topic != "will/topic" {
			t.Fatalf("expected will/topic, got %q", msg.GetPublish().Topic)
		}
	case <-time.After(time.Second):
		t.Fatal("expected delayed will to be published immediately when session expiry is zero")
	}
}

func TestCloseFallsBackToLocalWillDelayTimerWhenWillDelaySchedulingFails(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	willCh := make(chan *brokerpublish.Message, 1)
	subCenter := &recordingSubCenter{}
	willDelayCenter := &recordingWillDelayCenter{addErr: errors.New("will-delay center unavailable")}
	deliveryCursorStore := &recordingDeliveryCursorStore{}

	c := NewClient(
		&bufferConn{},
		WithSubCenter(subCenter),
		WithSessionCenter(&fakeSessionCenter{}),
		WithWillDelayCenter(willDelayCenter),
		WithNotifyWillMessageChan(willCh),
		WithDeliveryCursorStore(deliveryCursorStore),
	)
	c.ID = "client-b"
	c.ownerToken = "owner-token"
	c.ctx = context.Background()
	c.sessionExpiryInterval = uint32(2 * time.Hour / time.Second)
	c.connAckAccepted.Store(true)
	c.willMessage = &brokerpublish.Message{
		SendClientID: c.getID(),
		WillDelay:    120 * time.Millisecond,
		Publish: &packets.Publish{
			Topic:   "will/topic",
			Payload: []byte("bye"),
			QoS:     1,
		},
	}

	if err := c.close(); err != nil {
		t.Fatalf("close client: %v", err)
	}
	if got := willDelayCenter.addCount.Load(); got != 1 {
		t.Fatalf("expected will-delay scheduling to be attempted once, got %d", got)
	}

	select {
	case <-willCh:
		t.Fatal("will must not be published immediately when delayed scheduling fails")
	case <-time.After(40 * time.Millisecond):
	}

	select {
	case msg := <-willCh:
		if msg.GetPublish().Topic != "will/topic" {
			t.Fatalf("expected will/topic, got %q", msg.GetPublish().Topic)
		}
		if msg.OwnerToken != "owner-token" {
			t.Fatalf("expected owner token owner-token, got %q", msg.OwnerToken)
		}
	case <-time.After(time.Second):
		t.Fatal("expected local fallback timer to publish delayed will")
	}
}

func TestCloseSchedulesWillAtSessionExpiryUsingWillDelayCenter(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	willCh := make(chan *brokerpublish.Message, 1)
	willDelayCenter := &recordingWillDelayCenter{}

	c := NewClient(
		&bufferConn{},
		WithSubCenter(&recordingSubCenter{}),
		WithSessionCenter(&fakeSessionCenter{}),
		WithWillDelayCenter(willDelayCenter),
		WithNotifyWillMessageChan(willCh),
		WithDeliveryCursorStore(&recordingDeliveryCursorStore{}),
	)
	c.ID = "client-expiry"
	c.ownerToken = "owner-token"
	c.ctx = context.Background()
	c.sessionExpiryInterval = 1
	c.connAckAccepted.Store(true)
	c.willMessage = &brokerpublish.Message{
		SendClientID: c.getID(),
		WillDelay:    2 * time.Hour,
		Publish: &packets.Publish{
			Topic:   "will/topic",
			Payload: []byte("bye"),
			QoS:     1,
		},
	}

	start := time.Now()
	if err := c.close(); err != nil {
		t.Fatalf("close client: %v", err)
	}

	if got := willDelayCenter.addCount.Load(); got != 1 {
		t.Fatalf("expected one will-delay task for session-expiry scheduling, got %d", got)
	}
	task := willDelayCenter.LastTask()
	if task == nil {
		t.Fatal("expected scheduled will-delay task to be recorded")
	}
	fireAt := time.UnixMicro(task.GetScheduledPublishTime())
	delta := fireAt.Sub(start)
	if delta < 500*time.Millisecond || delta > 1500*time.Millisecond {
		t.Fatalf("expected will task around session expiry (1s), got delta=%s", delta)
	}
	select {
	case <-willCh:
		t.Fatal("will should not be published immediately when scheduled at session expiry")
	case <-time.After(60 * time.Millisecond):
	}
}

func TestScheduleWillDelayRequiresOwnerToken(t *testing.T) {
	logger.LoadForTest()

	willDelayCenter := &recordingWillDelayCenter{}
	c := NewClient(
		&bufferConn{},
		WithWillDelayCenter(willDelayCenter),
		WithNotifyWillMessageChan(make(chan *brokerpublish.Message, 1)),
	)
	c.ID = "client-no-token"
	c.ctx = context.Background()

	if c.scheduleWillDelay(context.Background(), time.Second) {
		t.Fatal("will delay scheduling must fail without owner token")
	}
	if got := willDelayCenter.addCount.Load(); got != 0 {
		t.Fatalf("expected no will-delay task without owner token, got %d", got)
	}
}

func TestCloseWithDisconnectNoWillSuppressesServerShutdownWill(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	willCh := make(chan *brokerpublish.Message, 1)
	c := NewClient(
		&bufferConn{},
		WithSessionCenter(&fakeSessionCenter{}),
		WithNotifyWillMessageChan(willCh),
	)
	c.ID = "client-shutdown"
	c.ctx = context.Background()
	c.sessionExpiryInterval = uint32(time.Hour / time.Second)
	c.connAckAccepted.Store(true)
	c.willMessage = &brokerpublish.Message{
		SendClientID: c.getID(),
		Publish: &packets.Publish{
			Topic:   "will/topic",
			Payload: []byte("bye"),
			QoS:     1,
		},
	}

	if err := c.CloseWithDisconnectNoWill(DisconnectForServerShuttingDown()); err != nil {
		t.Fatalf("close with server-shutdown disconnect: %v", err)
	}

	select {
	case msg := <-willCh:
		t.Fatalf("server-shutdown DISCONNECT must suppress will, got topic %q", msg.GetPublish().Topic)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestCloseWithDisconnectKeepWillReturnsConnCloseError(t *testing.T) {
	closeErr := errors.New("close failed")
	c := NewClient(&closeErrorConn{err: closeErr})
	c.ctx = context.Background()
	c.connAckAccepted.Store(true)

	err := c.CloseWithDisconnectKeepWill(DisconnectForQuotaExceeded("test close"))
	if !errors.Is(err, closeErr) {
		t.Fatalf("close error = %v, want %v", err, closeErr)
	}
}

func TestHandleConnectCleanStartDeletesDeliveryState(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	subCenter := &recordingSubCenter{}
	sessionCenter := &fakeSessionCenter{}
	deliveryCursorStore := &recordingDeliveryCursorStore{}
	c := NewClient(
		c1,
		WithSubCenter(subCenter),
		WithSessionCenter(sessionCenter),
		WithStateRouter(newTestInProcessStateRouter(t, sessionCenter, subCenter)),
		WithClientManager(NewManager()),
		WithDeliveryCursorStore(deliveryCursorStore),
	)
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	defer c.cancel(nil)

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
		if _, err := wire.Decode(c2, wire.DecodeOptions{}); err != nil && err != io.EOF {
			t.Errorf("read CONNACK: %v", err)
		}
	}()

	handler := NewClientHandler(c)
	if err := handler.handleConnect(&packets.Connect{
		ProtocolName:    "MQTT",
		ProtocolVersion: 5,
		ClientID:        "client-a",
		CleanStart:      true,
		KeepAlive:       30,
		Properties:      &packets.ConnectProperties{},
	}); err != nil {
		t.Fatalf("handleConnect: %v", err)
	}
	<-readDone

	if got := subCenter.deleteCount.Load(); got != 1 {
		t.Fatalf("expected subscriptions to be deleted once, got %d", got)
	}
	if got := deliveryCursorStore.deleteCount.Load(); got != 1 {
		t.Fatalf("expected delivery state to be deleted once, got %d", got)
	}
	if deliveryCursorStore.lastClient != "client-a" {
		t.Fatalf("expected delivery state delete for client-a, got %q", deliveryCursorStore.lastClient)
	}
}

func TestHandleConnectClosesLocalOldClientWithSessionTakenOverDisconnect(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	oldServer, oldPeer := net.Pipe()
	defer oldPeer.Close()
	newServer, newPeer := net.Pipe()
	defer newPeer.Close()

	manager := NewManager()
	sessionCenter := &fakeSessionCenter{}
	subCenter := &recordingSubCenter{}
	oldClient := NewClient(oldServer, WithSessionCenter(sessionCenter))
	oldClient.ID = "client-a"
	oldClient.ctx, oldClient.cancel = context.WithCancelCause(context.Background())
	oldClient.connAckAccepted.Store(true)
	manager.AddClient(oldClient.getID(), oldClient)

	oldPacketCh := make(chan *packets.ControlPacket, 1)
	oldErrCh := make(chan error, 1)
	go func() {
		_ = oldPeer.SetReadDeadline(time.Now().Add(2 * time.Second))
		cp, err := wire.Decode(oldPeer, wire.DecodeOptions{})
		if err != nil {
			oldErrCh <- err
			return
		}
		oldPacketCh <- cp
	}()

	newReadDone := make(chan struct{})
	go func() {
		defer close(newReadDone)
		_ = newPeer.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _ = wire.Decode(newPeer, wire.DecodeOptions{})
	}()

	newClient := NewClient(
		newServer,
		WithSessionCenter(sessionCenter),
		WithClientManager(manager),
		WithSubCenter(subCenter),
		WithStateRouter(newTestInProcessStateRouter(t, sessionCenter, subCenter)),
		WithDeliveryCursorStore(&recordingDeliveryCursorStore{}),
	)
	newClient.ctx, newClient.cancel = context.WithCancelCause(context.Background())

	if err := NewClientHandler(newClient).handleConnect(&packets.Connect{
		ProtocolName:    "MQTT",
		ProtocolVersion: 5,
		ClientID:        "client-a",
		CleanStart:      false,
		KeepAlive:       30,
		Properties:      &packets.ConnectProperties{},
	}); err != nil {
		t.Fatalf("handleConnect: %v", err)
	}
	<-newReadDone

	select {
	case cp := <-oldPacketCh:
		disc, ok := cp.Content.(*packets.Disconnect)
		if !ok {
			t.Fatalf("expected DISCONNECT for old client, got %T", cp.Content)
		}
		if disc.ReasonCode != packets.DisconnectSessionTakenOver {
			t.Fatalf("expected Session Taken Over reason 0x%x, got 0x%x", packets.DisconnectSessionTakenOver, disc.ReasonCode)
		}
	case err := <-oldErrCh:
		t.Fatalf("expected DISCONNECT before old connection close, got read error: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for old client DISCONNECT")
	}
}

type signalingSharedStore struct {
	unackedCalled chan struct{}
	closeOnce     sync.Once
}

func (s *signalingSharedStore) EnsureSchema(context.Context) error { return nil }
func (s *signalingSharedStore) AppendShareGroupTask(context.Context, time.Time, *sharedsubscription.ShareGroupTask) error {
	return nil
}
func (s *signalingSharedStore) ReadShareGroupTasks(context.Context, string, time.Time, uuid.UUID, int) ([]*sharedsubscription.ShareGroupTask, error) {
	return nil, nil
}
func (s *signalingSharedStore) MarkShareGroupTaskProcessed(context.Context, uuid.UUID, string) error {
	return nil
}
func (s *signalingSharedStore) AtomicUpdateTaskStatus(context.Context, uuid.UUID, string, sharedsubscription.TaskStatus, sharedsubscription.TaskStatus) (bool, error) {
	return true, nil
}
func (s *signalingSharedStore) GetUnAckedSharedSubscriptionTasks(context.Context, string, string, time.Time, uuid.UUID) ([]*sharedsubscription.ShareGroupTask, error) {
	s.closeOnce.Do(func() { close(s.unackedCalled) })
	return nil, nil
}
func (s *signalingSharedStore) RollbackSharedSubscriptionTask(context.Context, *sharedsubscription.ShareGroupTask) error {
	return nil
}
func (s *signalingSharedStore) QueryProcessingTasksBefore(context.Context, string, time.Time) ([]*sharedsubscription.ShareGroupTask, error) {
	return nil, nil
}
func (s *signalingSharedStore) CheckDeliveryTaskExists(context.Context, string, uuid.UUID) (bool, error) {
	return false, nil
}
func (s *signalingSharedStore) ReadShareGroupCursor(context.Context, string) (*sharedsubscription.ShareGroupCursor, error) {
	return &sharedsubscription.ShareGroupCursor{}, nil
}
func (s *signalingSharedStore) AppendShareGroupCursor(context.Context, string, *sharedsubscription.ShareGroupCursor) error {
	return nil
}
func (s *signalingSharedStore) QueryShareGroupTaskByMessageID(context.Context, string, uuid.UUID, []sharedsubscription.TaskStatus) (*sharedsubscription.ShareGroupTask, error) {
	return nil, nil
}

func TestCloseNotifiesSharedSubscriptionManagerOnOffline(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	sharedStore := &signalingSharedStore{unackedCalled: make(chan struct{})}
	mgr := shared_manager.NewSharedSubscriptionManager(
		sharedStore,
		nil,
		nil,
		nil,
		nil,
		&recordingDeliveryCursorStore{},
		&recordingSubCenter{},
		0,
		nil,
		nil,
	)
	defer mgr.Stop()
	if err := mgr.OnClientOnline(context.Background(), "client-a", "g", "a/#"); err != nil {
		t.Fatalf("OnClientOnline: %v", err)
	}

	c := NewClient(
		&bufferConn{},
		WithSessionCenter(&fakeSessionCenter{}),
		WithSharedSubscriptionManager(mgr),
	)
	c.ID = "client-a"
	c.ctx = context.Background()
	c.sessionExpiryInterval = 60

	if err := c.close(); err != nil {
		t.Fatalf("close client: %v", err)
	}

	select {
	case <-sharedStore.unackedCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("expected shared subscription manager to rollback client tasks on close")
	}
}

func TestCloseCancelsClientContext(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()
	c1, c2 := net.Pipe()
	defer c2.Close()

	c := NewClient(c1)
	c.ID = "client-close-cancel"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())

	if err := c.close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	select {
	case <-c.ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("client close must cancel client context so background runners can exit")
	}
}
