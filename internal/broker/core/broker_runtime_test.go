package core

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/broker/client"
	"github.com/BAN1ce/skyTree/internal/broker/delivery"
	"github.com/BAN1ce/skyTree/internal/broker/server"
	"github.com/BAN1ce/skyTree/logger"
	clientcap "github.com/BAN1ce/skyTree/pkg/brokerapi/clientcap"
	brokerstore "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	"github.com/BAN1ce/skyTree/pkg/cluster"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/wire"
	"github.com/BAN1ce/skyTree/pkg/retry"
	"github.com/BAN1ce/skyTree/proto/proto_session"
	"github.com/BAN1ce/skyTree/proto/proto_topic"
	"github.com/google/uuid"
)

type recordingClusterState struct {
	removeCalls int
	removeID    uint64
	removeErr   error
}

func (s *recordingClusterState) AddNode(context.Context, *cluster.NodeMeta) error {
	return nil
}

func (s *recordingClusterState) RemoveNode(_ context.Context, nodeID uint64) error {
	s.removeCalls++
	s.removeID = nodeID
	return s.removeErr
}

func (s *recordingClusterState) ListNode(context.Context) ([]*cluster.NodeMeta, error) {
	return nil, nil
}

func TestBrokerCloseRemovesNodeFromClusterState(t *testing.T) {
	logger.LoadForTest()

	state := &recordingClusterState{}
	b := &Broker{
		runtime: brokerRuntimeState{cancel: func() {}},
		network: brokerNetworkResources{server: &server.Server{}},
		clients: brokerClientResources{manager: client.NewManager()},
		cluster: brokerClusterResources{
			nodeState: state,
			nodeMeta: &cluster.NodeMeta{Cluster: config.Cluster{
				LocalNodeID: 7,
			}},
		},
	}

	_ = b.Close()

	if state.removeCalls != 1 {
		t.Fatalf("expected one RemoveNode call, got %d", state.removeCalls)
	}
	if state.removeID != 7 {
		t.Fatalf("expected node 7 to be removed, got %d", state.removeID)
	}
}

func TestBrokerCloseIgnoresRemoveNodeError(t *testing.T) {
	logger.LoadForTest()

	state := &recordingClusterState{removeErr: errors.New("remove failed")}
	b := &Broker{
		runtime: brokerRuntimeState{cancel: func() {}},
		network: brokerNetworkResources{server: &server.Server{}},
		clients: brokerClientResources{manager: client.NewManager()},
		cluster: brokerClusterResources{
			nodeState: state,
			nodeMeta: &cluster.NodeMeta{Cluster: config.Cluster{
				LocalNodeID: 9,
			}},
		},
	}

	_ = b.Close()

	if state.removeCalls != 1 {
		t.Fatalf("expected one RemoveNode call, got %d", state.removeCalls)
	}
}

type timeoutSessionCenter struct {
	removeReq *proto_session.RemoveOutgoingUnfinishedRequest
	removeErr error
}

func (c *timeoutSessionCenter) OpenSessionForConnect(context.Context, *proto_session.OpenSessionForConnectRequest) (*proto_session.OpenSessionForConnectResponse, error) {
	return nil, nil
}

func (c *timeoutSessionCenter) TakeOverSessionOwner(context.Context, *proto_session.TakeOverSessionOwnerRequest) (*proto_session.TakeOverSessionOwnerResponse, error) {
	return nil, nil
}

func (c *timeoutSessionCenter) ReplaceSessionStateOnCleanStart(context.Context, *proto_session.ReplaceSessionStateOnCleanStartRequest) error {
	return nil
}

func (c *timeoutSessionCenter) SaveOfflineState(context.Context, *proto_session.SaveOfflineStateRequest) error {
	return nil
}

func (c *timeoutSessionCenter) RemoveOutgoingUnfinished(_ context.Context, request *proto_session.RemoveOutgoingUnfinishedRequest) error {
	c.removeReq = request
	return c.removeErr
}

func (c *timeoutSessionCenter) UpsertIncomingUnfinished(context.Context, *proto_session.UpsertIncomingUnfinishedRequest) error {
	return nil
}

func (c *timeoutSessionCenter) RemoveIncomingUnfinished(context.Context, *proto_session.RemoveIncomingUnfinishedRequest) error {
	return nil
}

func (c *timeoutSessionCenter) CommitOutgoingProgress(context.Context, *proto_session.CommitOutgoingProgressRequest) error {
	return nil
}

func (c *timeoutSessionCenter) DeleteSession(context.Context, *proto_session.DeleteSessionRequest) error {
	return nil
}

func (c *timeoutSessionCenter) GetSession(context.Context, *proto_session.ReadSessionRequest) (*proto_session.ReadSessionResponse, error) {
	return nil, nil
}

func (c *timeoutSessionCenter) GetSessionOwner(context.Context, *proto_session.ReadSessionOwnerRequest) (*proto_session.ReadSessionOwnerResponse, error) {
	return nil, nil
}

func (c *timeoutSessionCenter) GetSessionOwners(context.Context, *proto_session.ReadSessionOwnersRequest) (*proto_session.ReadSessionOwnersResponse, error) {
	return &proto_session.ReadSessionOwnersResponse{Items: []*proto_session.ReadSessionOwnerItem{}}, nil
}

func readMQTTPacketWithTimeout(t *testing.T, conn net.Conn) (*packets.ControlPacket, error) {
	t.Helper()

	if err := conn.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	defer func() {
		_ = conn.SetReadDeadline(time.Time{})
	}()
	return wire.Decode(conn, wire.DecodeOptions{})
}

type expiringSessionCenter struct {
	timeoutSessionCenter
	deleted []string
}

func (c *expiringSessionCenter) DeleteExpiredSessions(context.Context, int64) ([]string, error) {
	return append([]string(nil), c.deleted...), nil
}

type brokerExpirySubCenter struct {
	deleted []string
}

func (s *brokerExpirySubCenter) CreateSub(context.Context, *proto_topic.SubRequest) (*proto_topic.SubResponse, error) {
	return &proto_topic.SubResponse{}, nil
}

func (s *brokerExpirySubCenter) DeleteSub(context.Context, *proto_topic.UnSubRequest) (*proto_topic.UnSubResponse, error) {
	return &proto_topic.UnSubResponse{}, nil
}

func (s *brokerExpirySubCenter) GetAllMatchTopics(context.Context, *proto_topic.GetAllMatchTopicsRequest) (*proto_topic.GetAllMatchTopicsResponse, error) {
	return &proto_topic.GetAllMatchTopicsResponse{}, nil
}

func (s *brokerExpirySubCenter) GetAllMatchTopicsForWildTopic(context.Context, *proto_topic.GetAllMatchTopicsForWildTopicRequest) (*proto_topic.GetAllMatchTopicsForWildTopicResponse, error) {
	return &proto_topic.GetAllMatchTopicsForWildTopicResponse{}, nil
}

func (s *brokerExpirySubCenter) DeleteClient(_ context.Context, req *proto_topic.DeleteClientRequest) (*proto_topic.DeleteClientResponse, error) {
	s.deleted = append(s.deleted, req.GetClientID())
	return &proto_topic.DeleteClientResponse{Success: true}, nil
}

func (s *brokerExpirySubCenter) DeleteTopic(context.Context, *proto_topic.DeleteTopicRequest) (*proto_topic.DeleteTopicResponse, error) {
	return &proto_topic.DeleteTopicResponse{}, nil
}

func (s *brokerExpirySubCenter) GetAllMatchClient(context.Context, *proto_topic.GetAllSubTopicClientRequest) (*proto_topic.GetAllSubTopicClientResponse, error) {
	return &proto_topic.GetAllSubTopicClientResponse{}, nil
}

func (s *brokerExpirySubCenter) GetAllMatchClientV2(context.Context, *proto_topic.GetAllMatchClientV2Request) (*proto_topic.GetAllMatchClientV2Response, error) {
	return &proto_topic.GetAllMatchClientV2Response{}, nil
}

func (s *brokerExpirySubCenter) GetSubTree(context.Context, *proto_topic.GetSubTreeRequest) (*proto_topic.GetSubTreeResponse, error) {
	return &proto_topic.GetSubTreeResponse{}, nil
}

func (s *brokerExpirySubCenter) SetClientOwnerToken(context.Context, *proto_topic.SetClientOwnerTokenRequest) (*proto_topic.SetClientOwnerTokenResponse, error) {
	return &proto_topic.SetClientOwnerTokenResponse{Success: true}, nil
}

func (s *brokerExpirySubCenter) GetClientSubscriptions(context.Context, *proto_topic.GetClientSubscriptionsRequest) (*proto_topic.GetClientSubscriptionsResponse, error) {
	return &proto_topic.GetClientSubscriptionsResponse{}, nil
}

func (s *brokerExpirySubCenter) GetShareGroupMembers(context.Context, *proto_topic.GetShareGroupMembersRequest) (*proto_topic.GetShareGroupMembersResponse, error) {
	return &proto_topic.GetShareGroupMembersResponse{}, nil
}

type brokerExpiryCursorStore struct {
	deleted []string
}

var _ delivery.CursorStore = (*brokerExpiryCursorStore)(nil)
var _ delivery.ClientStateDeleter = (*brokerExpiryCursorStore)(nil)

func (s *brokerExpiryCursorStore) ReadCursor(context.Context, string) (*brokerstore.DeliveryCursor, error) {
	return nil, nil
}

func (s *brokerExpiryCursorStore) AdvanceCursor(context.Context, brokerstore.DeliveryCursor) error {
	return nil
}

func (s *brokerExpiryCursorStore) ReadTasks(context.Context, string, time.Time, uuid.UUID, int) ([]*brokerstore.DeliveryTask, error) {
	return nil, nil
}

func (s *brokerExpiryCursorStore) LoadMessagePayload(context.Context, uuid.UUID) ([]byte, error) {
	return nil, brokerstore.ErrMessagePayloadNotFound
}

func (s *brokerExpiryCursorStore) DeleteClientState(_ context.Context, clientID string) error {
	s.deleted = append(s.deleted, clientID)
	return nil
}

func TestBrokerDeleteExpiredSessionsCleansSubscriptionsAndDeliveryState(t *testing.T) {
	center := &expiringSessionCenter{deleted: []string{"client-a", "client-b"}}
	subCenter := &brokerExpirySubCenter{}
	cursorStore := &brokerExpiryCursorStore{}
	b := &Broker{
		state: brokerStateCenters{
			sessionCenter: center,
			subCenter:     subCenter,
		},
		delivery: brokerDeliveryResources{cursorStore: cursorStore},
	}

	b.deleteExpiredSessionsOnce(context.Background(), time.Now())

	if got, want := subCenter.deleted, []string{"client-a", "client-b"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("deleted subscriptions = %v, want %v", got, want)
	}
	if got, want := cursorStore.deleted, []string{"client-a", "client-b"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("deleted delivery states = %v, want %v", got, want)
	}
}

func TestBrokerCallTimeoutPreservesOutgoingUnfinished(t *testing.T) {
	center := &timeoutSessionCenter{}
	b := &Broker{state: brokerStateCenters{sessionCenter: center}}
	messageID := uuid.New()
	task := &retry.Task{
		Key: "retry-key",
		Data: &brokerpublish.Message{
			SendClientID: "client-a",
			MessageID:    messageID,
			RetryInfo: &brokerpublish.RetryInfo{
				Key:          "retry-key",
				FirstPubTime: time.Now(),
				Timeout:      time.Second,
			},
		},
	}

	if err := b.CallTimeout(task); err != nil {
		t.Fatalf("CallTimeout returned error: %v", err)
	}
	if center.removeReq != nil {
		t.Fatalf("expected outgoing unfinished to be preserved, got cleanup request for %q", center.removeReq.GetMessageID())
	}
}

func TestBrokerCallTimeoutDisconnectsOnlineClient(t *testing.T) {
	logger.LoadForTest()

	serverConn, peerConn := net.Pipe()
	defer func() {
		_ = peerConn.Close()
	}()

	onlineClient := client.NewClient(serverConn)
	onlineClient.ID = "client-a"
	manager := client.NewManager()
	manager.AddClient("client-a", onlineClient)
	center := &timeoutSessionCenter{}
	b := &Broker{
		clients: brokerClientResources{manager: manager},
		state:   brokerStateCenters{sessionCenter: center},
	}

	connAckDone := make(chan error, 1)
	go func() {
		connAck := packets.NewControlPacket(packets.CONNACK)
		connAck.Content = &packets.ConnAck{}
		connAckDone <- onlineClient.Write(&clientcap.WritePacket{Packet: connAck})
	}()
	if _, err := readMQTTPacketWithTimeout(t, peerConn); err != nil {
		t.Fatalf("read CONNACK: %v", err)
	}
	if err := <-connAckDone; err != nil {
		t.Fatalf("write CONNACK: %v", err)
	}

	messageID := uuid.New()
	task := &retry.Task{
		Key: "retry-key",
		Data: &brokerpublish.Message{
			SendClientID: "client-a",
			MessageID:    messageID,
			RetryInfo: &brokerpublish.RetryInfo{
				Key:          "retry-key",
				FirstPubTime: time.Now().Add(-time.Minute),
				Timeout:      time.Second,
			},
		},
	}

	timeoutDone := make(chan error, 1)
	go func() {
		timeoutDone <- b.CallTimeout(task)
	}()

	cp, err := readMQTTPacketWithTimeout(t, peerConn)
	if err != nil {
		t.Fatalf("read DISCONNECT: %v", err)
	}
	disconnect, ok := cp.Content.(*packets.Disconnect)
	if !ok {
		t.Fatalf("expected DISCONNECT, got %T", cp.Content)
	}
	if disconnect.ReasonCode != packets.DisconnectQuotaExceeded {
		t.Fatalf("disconnect reason = 0x%X, want 0x%X", disconnect.ReasonCode, packets.DisconnectQuotaExceeded)
	}
	if err := <-timeoutDone; err != nil {
		t.Fatalf("CallTimeout returned error: %v", err)
	}
	if center.removeReq != nil {
		t.Fatalf("expected outgoing unfinished to be preserved, got cleanup request for %q", center.removeReq.GetMessageID())
	}
}

func TestBrokerCallTimeoutDoesNotTouchSessionCleanup(t *testing.T) {
	center := &timeoutSessionCenter{removeErr: errors.New("cleanup failed")}
	b := &Broker{state: brokerStateCenters{sessionCenter: center}}
	task := &retry.Task{
		Data: &brokerpublish.Message{
			SendClientID: "client-a",
			MessageID:    uuid.New(),
		},
	}

	if err := b.CallTimeout(task); err != nil {
		t.Fatalf("CallTimeout returned error: %v", err)
	}
	if center.removeReq != nil {
		t.Fatalf("expected session cleanup not to be called, got request for %q", center.removeReq.GetMessageID())
	}
}

func TestBrokerCallRetryIgnoresInvalidTask(t *testing.T) {
	b := &Broker{clients: brokerClientResources{manager: client.NewManager()}}

	cases := []*retry.Task{
		nil,
		{},
		{Data: &brokerpublish.Message{}},
		{ClientID: "client-a"},
	}
	for idx, task := range cases {
		if err := b.CallRetry(task); err != nil {
			t.Fatalf("case %d: expected nil error for invalid task, got %v", idx, err)
		}
	}
}

func TestBrokerCloseDrainsWillMessages(t *testing.T) {
	b := &Broker{
		runtime: brokerRuntimeState{cancel: func() {}},
		clients: brokerClientResources{manager: client.NewManager()},
		will:    brokerWillResources{messageChan: make(chan *brokerpublish.Message, 2)},
	}
	b.will.messageChan <- &brokerpublish.Message{}

	if err := b.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if pending := len(b.will.messageChan); pending != 0 {
		t.Fatalf("expected will message channel drained, pending=%d", pending)
	}
}

func TestListenWillMessageDrainsAfterContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	b := &Broker{
		runtime: brokerRuntimeState{ctx: ctx},
		will:    brokerWillResources{messageChan: make(chan *brokerpublish.Message, 2)},
	}
	b.will.messageChan <- &brokerpublish.Message{}

	done := make(chan struct{})
	go func() {
		b.listenWillMessage()
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("listenWillMessage did not exit")
	}
	if pending := len(b.will.messageChan); pending != 0 {
		t.Fatalf("expected will message channel drained after context cancel, pending=%d", pending)
	}
}
