package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/broker/retain"
	"github.com/BAN1ce/skyTree/logger"
	brokerstore "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/wire"
	proto_topic "github.com/BAN1ce/skyTree/proto/proto_topic"
	"github.com/rs/zerolog"
)

func initClientPackageConfig(t *testing.T) {
	t.Helper()
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
}

type retainedMatchSubCenter struct {
	recordingSubCenter
	matches []*proto_topic.ClientMatch
}

func (s *retainedMatchSubCenter) GetAllMatchClientV2(context.Context, *proto_topic.GetAllMatchClientV2Request) (*proto_topic.GetAllMatchClientV2Response, error) {
	return &proto_topic.GetAllMatchClientV2Response{Matches: s.matches}, nil
}

func TestRetainedSubscriptionOptionsFilterNoLocalSelfMatches(t *testing.T) {
	c := NewClient(&callbackConn{})
	c.ID = "c1"
	c.component.subCenter = &retainedMatchSubCenter{
		matches: []*proto_topic.ClientMatch{
			{
				ClientID: "c1",
				Matched: []*proto_topic.MatchedSubscription{
					{TopicFilter: "a/#", QoS: 2, NoLocal: true, SubscriptionIdentifier: 10},
					{TopicFilter: "a/b", QoS: 1, NoLocal: false, RetainAsPublished: true, SubscriptionIdentifier: 20},
				},
			},
		},
	}

	qos, rap, subIDs, ok := NewClientHandler(c).getRetainedMessageSubscriptionOptions(context.Background(), "a/b", "c1")
	if !ok {
		t.Fatal("expected remaining retained subscription match")
	}
	if qos != 1 {
		t.Fatalf("expected QoS from remaining subscription, got %d", qos)
	}
	if !rap {
		t.Fatal("expected default RAP true")
	}
	if len(subIDs) != 1 || subIDs[0] != 20 {
		t.Fatalf("expected only remaining subscription id [20], got %v", subIDs)
	}
}

func TestRetainedSubscriptionOptionsDropWhenAllSelfMatchesNoLocal(t *testing.T) {
	c := NewClient(&callbackConn{})
	c.ID = "c1"
	c.component.subCenter = &retainedMatchSubCenter{
		matches: []*proto_topic.ClientMatch{
			{
				ClientID: "c1",
				Matched: []*proto_topic.MatchedSubscription{
					{TopicFilter: "a/#", QoS: 1, NoLocal: true, SubscriptionIdentifier: 10},
				},
			},
		},
	}

	_, _, _, ok := NewClientHandler(c).getRetainedMessageSubscriptionOptions(context.Background(), "a/b", "c1")
	if ok {
		t.Fatal("expected all retained matches to be filtered by NoLocal")
	}
}

func TestSendRetainedQoS1RegistersOutgoingInflight(t *testing.T) {
	initClientPackageConfig(t)
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	retainedStore := retain.NewRetainStore(newClientRetainMemHashStore())
	publish := &packets.Publish{
		Topic:    "a/b",
		QoS:      1,
		PacketID: 9,
		Retain:   true,
		Payload:  []byte("retained"),
	}
	retained, err := newRetainMessageFromPublish(publish, time.Now(), "publisher")
	if err != nil {
		t.Fatalf("new retain message: %v", err)
	}
	if err := retainedStore.PutRetainMessage(retained); err != nil {
		t.Fatalf("put retain message: %v", err)
	}

	conn := &callbackConn{}
	c := NewClient(conn)
	c.ID = "c1"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.component.retain = retainedStore
	c.component.subCenter = &retainedMatchSubCenter{
		matches: []*proto_topic.ClientMatch{
			{
				ClientID: "c1",
				Matched: []*proto_topic.MatchedSubscription{
					{TopicFilter: "a/b", QoS: 1, RetainAsPublished: true, SubscriptionIdentifier: 7},
				},
			},
		},
	}

	handler := NewClientHandler(c)
	if _, ok := retainedStore.GetRetainMessage("a/b"); !ok {
		t.Fatal("expected retained message in store")
	}
	if msg := handler.getRetainMessage("a/b"); msg == nil || msg.GetControlPacket() == nil {
		t.Fatal("expected retained message to convert to publish")
	}
	if _, _, _, ok := handler.getRetainedMessageSubscriptionOptions(context.Background(), "a/b", "publisher"); !ok {
		t.Fatal("expected retained subscription options")
	}

	handler.sendRetainedForTopic(context.Background(), "a/b")

	cp, err := wire.Decode(conn, wire.DecodeOptions{})
	if err != nil {
		t.Fatalf("read retained publish: %v", err)
	}
	pub, ok := cp.Content.(*packets.Publish)
	if !ok {
		t.Fatalf("expected PUBLISH, got %T", cp.Content)
	}
	if pub.QoS != 1 || pub.PacketID == 0 {
		t.Fatalf("expected retained QoS1 with packet id, got qos=%d packetID=%d", pub.QoS, pub.PacketID)
	}
	if !pub.Retain {
		t.Fatal("expected retained publish replay to keep retain flag")
	}
	if pub.Properties == nil {
		t.Fatal("expected publish properties")
	}
	rawIDs, _ := json.Marshal(pub.Properties.SubscriptionIdentifier)
	if string(rawIDs) != "[7]" {
		t.Fatalf("expected subscription id [7], got %s", rawIDs)
	}

	if c.outgoingInflight.Len() != 1 {
		t.Fatalf("expected retained QoS1 to be tracked as outgoing inflight, got %d", c.outgoingInflight.Len())
	}
}

func TestSendRetainedAppliesRetainAsPublishedFalse(t *testing.T) {
	initClientPackageConfig(t)
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	retainedStore := retain.NewRetainStore(newClientRetainMemHashStore())
	publish := &packets.Publish{
		Topic:   "a/b",
		QoS:     0,
		Retain:  true,
		Payload: []byte("retained"),
	}
	retained, err := newRetainMessageFromPublish(publish, time.Now(), "publisher")
	if err != nil {
		t.Fatalf("new retain message: %v", err)
	}
	if err := retainedStore.PutRetainMessage(retained); err != nil {
		t.Fatalf("put retain message: %v", err)
	}

	conn := &callbackConn{}
	c := NewClient(conn)
	c.ID = "c1"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	defer c.cancel(nil)
	c.component.retain = retainedStore
	c.component.subCenter = &retainedMatchSubCenter{
		matches: []*proto_topic.ClientMatch{
			{
				ClientID: "c1",
				Matched: []*proto_topic.MatchedSubscription{
					{TopicFilter: "a/b", QoS: 0, RetainAsPublished: false},
				},
			},
		},
	}

	NewClientHandler(c).sendRetainedForTopic(context.Background(), "a/b")

	cp, err := wire.Decode(conn, wire.DecodeOptions{})
	if err != nil {
		t.Fatalf("read retained publish: %v", err)
	}
	pub, ok := cp.Content.(*packets.Publish)
	if !ok {
		t.Fatalf("expected PUBLISH, got %T", cp.Content)
	}
	if pub.Retain {
		t.Fatal("expected RAP=false retained replay to clear retain flag")
	}
	if string(pub.Payload) != "retained" {
		t.Fatalf("payload = %q, want retained", pub.Payload)
	}
}

func TestSendRetainedAfterSubscribeSkipsWhenRetainUnavailable(t *testing.T) {
	initClientPackageConfig(t)
	cfg := mustLoadConfigForTest(t)
	oldRetainAvailable := cfg.Broker.ConnectAckProperty.RetainAvailable
	cfg.Broker.ConnectAckProperty.RetainAvailable = 0
	defer func() {
		cfg.Broker.ConnectAckProperty.RetainAvailable = oldRetainAvailable
	}()
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	retainedStore := retain.NewRetainStore(newClientRetainMemHashStore())
	publish := &packets.Publish{
		Topic:   "a/b",
		QoS:     0,
		Retain:  true,
		Payload: []byte("retained"),
	}
	retained, err := newRetainMessageFromPublish(publish, time.Now(), "publisher")
	if err != nil {
		t.Fatalf("new retain message: %v", err)
	}
	if err := retainedStore.PutRetainMessage(retained); err != nil {
		t.Fatalf("put retain message: %v", err)
	}

	conn := &callbackConn{}
	c := NewClient(conn, WithConfig(clientConfigForTest(cfg.Broker)))
	c.ID = "c1"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	defer c.cancel(nil)
	c.component.retain = retainedStore
	c.component.subCenter = &retainedMatchSubCenter{
		matches: []*proto_topic.ClientMatch{
			{
				ClientID: "c1",
				Matched: []*proto_topic.MatchedSubscription{
					{TopicFilter: "a/b", QoS: 0},
				},
			},
		},
	}

	NewClientHandler(c).sendRetainedAfterSubscribe(context.Background(), &packets.Subscribe{
		Subscriptions: []packets.SubOptions{{Topic: "a/b"}},
	}, nil)

	if conn.Len() != 0 {
		t.Fatalf("expected retained publish to be skipped when RetainAvailable=0, wrote %d bytes", conn.Len())
	}
}

func TestSendRetainedAfterSubscribeRetainHandlingIfNewDoesNotDuplicateInSingleSubscribe(t *testing.T) {
	initClientPackageConfig(t)
	logger.Logger = &logger.SkyLogger{Logger: zerolog.New(io.Discard)}

	retainedStore := retain.NewRetainStore(newClientRetainMemHashStore())
	publish := &packets.Publish{
		Topic:   "a/b",
		QoS:     0,
		Retain:  true,
		Payload: []byte("retained"),
	}
	retained, err := newRetainMessageFromPublish(publish, time.Now(), "publisher")
	if err != nil {
		t.Fatalf("new retain message: %v", err)
	}
	if err := retainedStore.PutRetainMessage(retained); err != nil {
		t.Fatalf("put retain message: %v", err)
	}

	conn := &callbackConn{}
	c := NewClient(conn)
	c.ID = "c1"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	defer c.cancel(nil)
	c.component.retain = retainedStore
	c.component.subCenter = &retainedMatchSubCenter{
		matches: []*proto_topic.ClientMatch{
			{
				ClientID: "c1",
				Matched: []*proto_topic.MatchedSubscription{
					{TopicFilter: "a/b", QoS: 0},
				},
			},
		},
	}

	NewClientHandler(c).sendRetainedAfterSubscribe(context.Background(), &packets.Subscribe{
		Subscriptions: []packets.SubOptions{
			{Topic: "a/b", RetainHandling: packets.RetainSendOnSubscribeIfNew},
			{Topic: "a/b", RetainHandling: packets.RetainSendOnSubscribeIfNew},
		},
	}, map[string]bool{})

	reader := bytes.NewReader(conn.Bytes())
	packetCount := 0
	for reader.Len() > 0 {
		cp, err := wire.Decode(reader, wire.DecodeOptions{})
		if err != nil {
			t.Fatalf("decode retained publish #%d: %v", packetCount+1, err)
		}
		if _, ok := cp.Content.(*packets.Publish); !ok {
			t.Fatalf("expected retained PUBLISH, got %T", cp.Content)
		}
		packetCount++
	}
	if packetCount != 1 {
		t.Fatalf("expected one retained PUBLISH for duplicated RH=1 subscriptions, got %d", packetCount)
	}
}

type clientRetainMemHashStore struct {
	hashes map[string]map[string][]byte
}

func newClientRetainMemHashStore() *clientRetainMemHashStore {
	return &clientRetainMemHashStore{hashes: make(map[string]map[string][]byte)}
}

func (m *clientRetainMemHashStore) HSet(_ context.Context, key []byte, field [][]byte) error {
	if len(field) < 2 {
		return nil
	}
	k := string(key)
	if m.hashes[k] == nil {
		m.hashes[k] = make(map[string][]byte)
	}
	m.hashes[k][string(field[0])] = append([]byte(nil), field[1]...)
	return nil
}

func (m *clientRetainMemHashStore) HGet(_ context.Context, key, field []byte) ([]byte, bool, error) {
	v, ok := m.hashes[string(key)][string(field)]
	return append([]byte(nil), v...), ok, nil
}

func (m *clientRetainMemHashStore) HDel(_ context.Context, key []byte, field [][]byte) error {
	for _, f := range field {
		delete(m.hashes[string(key)], string(f))
	}
	return nil
}

func (m *clientRetainMemHashStore) HGetAll(_ context.Context, key []byte) (map[string]string, error) {
	out := make(map[string]string, len(m.hashes[string(key)]))
	for field, value := range m.hashes[string(key)] {
		out[field] = string(value)
	}
	return out, nil
}

func (m *clientRetainMemHashStore) HPrefix(context.Context, []byte, []byte) (map[string]string, error) {
	return map[string]string{}, nil
}

func (m *clientRetainMemHashStore) DeleteHash(_ context.Context, key []byte) error {
	delete(m.hashes, string(key))
	return nil
}

var _ brokerstore.HashStore = (*clientRetainMemHashStore)(nil)
