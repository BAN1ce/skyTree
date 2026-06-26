package staterouter

import (
	"context"
	"errors"
	"testing"

	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/session"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/subscription"
	"github.com/BAN1ce/skyTree/pkg/cluster"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/proto/proto_session"
	"github.com/BAN1ce/skyTree/proto/proto_topic"
)

// ---------------------------------------------------------------------------
// Test fakes. Each embeds the real interface so only the methods exercised by
// a test need to be implemented; any unexpected call hits the nil embedded
// value and panics, which surfaces the mistake.
// ---------------------------------------------------------------------------

type fakeSessionCenter struct {
	session.Center

	calls *[]string

	openResp *proto_session.OpenSessionForConnectResponse
	openErr  error

	cleanStartErr error

	takeoverReq  *proto_session.TakeOverSessionOwnerRequest
	takeoverResp *proto_session.TakeOverSessionOwnerResponse
	takeoverErr  error

	saveReq *proto_session.SaveOfflineStateRequest
	saveErr error
}

func (f *fakeSessionCenter) OpenSessionForConnect(
	_ context.Context,
	_ *proto_session.OpenSessionForConnectRequest,
) (*proto_session.OpenSessionForConnectResponse, error) {
	if f.calls != nil {
		*f.calls = append(*f.calls, "open")
	}
	return f.openResp, f.openErr
}

func (f *fakeSessionCenter) ReplaceSessionStateOnCleanStart(
	_ context.Context,
	_ *proto_session.ReplaceSessionStateOnCleanStartRequest,
) error {
	if f.calls != nil {
		*f.calls = append(*f.calls, "cleanStart")
	}
	return f.cleanStartErr
}

func (f *fakeSessionCenter) TakeOverSessionOwner(
	_ context.Context,
	req *proto_session.TakeOverSessionOwnerRequest,
) (*proto_session.TakeOverSessionOwnerResponse, error) {
	if f.calls != nil {
		*f.calls = append(*f.calls, "takeover")
	}
	f.takeoverReq = req
	return f.takeoverResp, f.takeoverErr
}

func (f *fakeSessionCenter) SaveOfflineState(
	_ context.Context,
	req *proto_session.SaveOfflineStateRequest,
) error {
	f.saveReq = req
	return f.saveErr
}

type fakeSubscriptionCenter struct {
	subscription.Center

	calls *[]string

	setTokenErr error

	createReqs []*proto_topic.SubRequest
	createResp *proto_topic.SubResponse
	createErr  error

	deleteReq *proto_topic.DeleteClientRequest
	deleteErr error

	listResp *proto_topic.GetClientSubscriptionsResponse
	listErr  error
}

func (f *fakeSubscriptionCenter) SetClientOwnerToken(
	_ context.Context,
	_ *proto_topic.SetClientOwnerTokenRequest,
) (*proto_topic.SetClientOwnerTokenResponse, error) {
	if f.calls != nil {
		*f.calls = append(*f.calls, "setOwnerToken")
	}
	return &proto_topic.SetClientOwnerTokenResponse{}, f.setTokenErr
}

func (f *fakeSubscriptionCenter) CreateSub(
	_ context.Context,
	req *proto_topic.SubRequest,
) (*proto_topic.SubResponse, error) {
	f.createReqs = append(f.createReqs, req)
	return f.createResp, f.createErr
}

func (f *fakeSubscriptionCenter) DeleteClient(
	_ context.Context,
	req *proto_topic.DeleteClientRequest,
) (*proto_topic.DeleteClientResponse, error) {
	f.deleteReq = req
	return &proto_topic.DeleteClientResponse{}, f.deleteErr
}

func (f *fakeSubscriptionCenter) GetClientSubscriptions(
	_ context.Context,
	_ *proto_topic.GetClientSubscriptionsRequest,
) (*proto_topic.GetClientSubscriptionsResponse, error) {
	return f.listResp, f.listErr
}

type fakeNodeController struct {
	cluster.NodeController

	nodeID     uint64
	clientID   string
	ownerToken string
	called     bool
	err        error
}

func (f *fakeNodeController) RequestCloseClient(
	_ context.Context,
	nodeID uint64,
	clientID string,
	ownerToken string,
) error {
	f.called = true
	f.nodeID = nodeID
	f.clientID = clientID
	f.ownerToken = ownerToken
	return f.err
}

// noopPublishRoute is a PublishRoute handler that does nothing, for wiring
// clients in tests that don't exercise RoutePublish.
func noopPublishRoute(context.Context, RoutePublishRequest) error { return nil }

// newTestClient builds an InProcessClient directly from fakes, bypassing
// NewInProcessClient so individual collaborators can be nil when unused.
func newTestClient(
	sc session.Center,
	subc subscription.Center,
	nc cluster.NodeController,
	route PublishRouteHandler,
) *InProcessClient {
	return &InProcessClient{
		sessionCenter:      sc,
		subscriptionCenter: subc,
		nodeController:     nc,
		publishRoute:       route,
	}
}

// ---------------------------------------------------------------------------
// NewInProcessClient
// ---------------------------------------------------------------------------

func TestNewInProcessClient(t *testing.T) {
	full := InProcessDependencies{
		SessionCenter:      &fakeSessionCenter{},
		SubscriptionCenter: &fakeSubscriptionCenter{},
		NodeController:     &fakeNodeController{},
		PublishRoute:       noopPublishRoute,
	}

	t.Run("valid", func(t *testing.T) {
		c, err := NewInProcessClient(full)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c == nil {
			t.Fatal("expected non-nil client")
		}
	})

	t.Run("nil session center", func(t *testing.T) {
		deps := full
		deps.SessionCenter = nil
		if _, err := NewInProcessClient(deps); err == nil {
			t.Fatal("expected error for nil session center")
		}
	})

	t.Run("nil subscription center", func(t *testing.T) {
		deps := full
		deps.SubscriptionCenter = nil
		if _, err := NewInProcessClient(deps); err == nil {
			t.Fatal("expected error for nil subscription center")
		}
	})

	t.Run("nil publish route", func(t *testing.T) {
		deps := full
		deps.PublishRoute = nil
		if _, err := NewInProcessClient(deps); err == nil {
			t.Fatal("expected error for nil publish route")
		}
	})

	t.Run("nil node controller is allowed", func(t *testing.T) {
		deps := full
		deps.NodeController = nil
		if _, err := NewInProcessClient(deps); err != nil {
			t.Fatalf("node controller should be optional, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// NewRoutePublishHandler
// ---------------------------------------------------------------------------

func TestNewRoutePublishHandler(t *testing.T) {
	if _, err := NewRoutePublishHandler(nil); err == nil {
		t.Fatal("expected error for nil handler")
	}
	h, err := NewRoutePublishHandler(noopPublishRoute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}

// ---------------------------------------------------------------------------
// subscribeWithQoSCap (pure)
// ---------------------------------------------------------------------------

func TestSubscribeWithQoSCap(t *testing.T) {
	t.Run("nil packet", func(t *testing.T) {
		if got := subscribeWithQoSCap(nil, 1); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("caps qos and leaves original untouched", func(t *testing.T) {
		original := &packets.Subscribe{
			Subscriptions: []packets.SubOptions{
				{Topic: "a", QoS: 2},
				{Topic: "b", QoS: 0},
			},
		}
		capped := subscribeWithQoSCap(original, 1)
		if capped.Subscriptions[0].QoS != 1 {
			t.Fatalf("expected QoS capped to 1, got %d", capped.Subscriptions[0].QoS)
		}
		if capped.Subscriptions[1].QoS != 0 {
			t.Fatalf("expected QoS 0 preserved, got %d", capped.Subscriptions[1].QoS)
		}
		if original.Subscriptions[0].QoS != 2 {
			t.Fatalf("original packet must not be mutated, got %d", original.Subscriptions[0].QoS)
		}
	})

	t.Run("out-of-range max returns packet unchanged", func(t *testing.T) {
		original := &packets.Subscribe{
			Subscriptions: []packets.SubOptions{{Topic: "a", QoS: 2}},
		}
		if got := subscribeWithQoSCap(original, 5); got != original {
			t.Fatal("expected same packet for out-of-range maxQoS")
		}
		if got := subscribeWithQoSCap(original, -1); got != original {
			t.Fatal("expected same packet for negative maxQoS")
		}
	})
}

// ---------------------------------------------------------------------------
// validation helpers
// ---------------------------------------------------------------------------

func TestValidateClientOwnerRequest(t *testing.T) {
	cases := []struct {
		name       string
		clientID   string
		ownerToken string
		nodeID     uint64
		wantErr    bool
	}{
		{"ok", "c1", "tok", 1, false},
		{"empty client id", "", "tok", 1, true},
		{"empty owner token", "c1", "", 1, true},
		{"zero node id", "c1", "tok", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateClientOwnerRequest(tc.clientID, tc.ownerToken, tc.nodeID)
			if (err != nil) != tc.wantErr {
				t.Fatalf("wantErr=%v got %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidateAcquireSessionRequest(t *testing.T) {
	base := AcquireSessionRequest{ClientID: "c1", OwnerToken: "tok", BrokerNodeID: 1, NowUnixNano: 1}
	if err := validateAcquireSessionRequest(base); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bad := base
	bad.NowUnixNano = 0
	if err := validateAcquireSessionRequest(bad); err == nil {
		t.Fatal("expected error for zero NowUnixNano")
	}
}

// ---------------------------------------------------------------------------
// AcquireSession
// ---------------------------------------------------------------------------

func TestAcquireSession(t *testing.T) {
	baseReq := AcquireSessionRequest{
		BrokerNodeID:          7,
		ClientID:              "c1",
		OwnerToken:            "tok-1",
		SessionExpiryInterval: 60,
		NowUnixNano:           1234,
	}

	t.Run("ordering without clean start", func(t *testing.T) {
		var calls []string
		sc := &fakeSessionCenter{
			calls:        &calls,
			openResp:     &proto_session.OpenSessionForConnectResponse{},
			takeoverResp: &proto_session.TakeOverSessionOwnerResponse{},
		}
		subc := &fakeSubscriptionCenter{calls: &calls}
		c := newTestClient(sc, subc, nil, noopPublishRoute)

		resp, err := c.AcquireSession(context.Background(), baseReq)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp == nil || resp.OpenSession == nil || resp.Takeover == nil {
			t.Fatal("expected populated response")
		}
		want := []string{"open", "takeover", "setOwnerToken"}
		assertCalls(t, calls, want)

		// Owner identity is passed through to takeover.
		owner := sc.takeoverReq.GetOwner()
		if owner.GetNodeID() != 7 || owner.GetOwnerToken() != "tok-1" || !owner.GetOnline() {
			t.Fatalf("takeover owner mismatch: %+v", owner)
		}
	})

	t.Run("clean start inserts reset step", func(t *testing.T) {
		var calls []string
		sc := &fakeSessionCenter{
			calls:        &calls,
			openResp:     &proto_session.OpenSessionForConnectResponse{},
			takeoverResp: &proto_session.TakeOverSessionOwnerResponse{},
		}
		subc := &fakeSubscriptionCenter{calls: &calls}
		c := newTestClient(sc, subc, nil, noopPublishRoute)

		req := baseReq
		req.CleanStart = true
		if _, err := c.AcquireSession(context.Background(), req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertCalls(t, calls, []string{"open", "cleanStart", "takeover", "setOwnerToken"})
	})

	t.Run("validation failure short-circuits", func(t *testing.T) {
		var calls []string
		sc := &fakeSessionCenter{calls: &calls}
		subc := &fakeSubscriptionCenter{calls: &calls}
		c := newTestClient(sc, subc, nil, noopPublishRoute)

		req := baseReq
		req.OwnerToken = ""
		if _, err := c.AcquireSession(context.Background(), req); err == nil {
			t.Fatal("expected validation error")
		}
		if len(calls) != 0 {
			t.Fatalf("expected no downstream calls, got %v", calls)
		}
	})

	t.Run("propagates open error", func(t *testing.T) {
		sentinel := errors.New("open boom")
		sc := &fakeSessionCenter{openErr: sentinel}
		subc := &fakeSubscriptionCenter{}
		c := newTestClient(sc, subc, nil, noopPublishRoute)
		if _, err := c.AcquireSession(context.Background(), baseReq); !errors.Is(err, sentinel) {
			t.Fatalf("expected open error, got %v", err)
		}
	})

	t.Run("propagates takeover error", func(t *testing.T) {
		sentinel := errors.New("takeover boom")
		sc := &fakeSessionCenter{
			openResp:    &proto_session.OpenSessionForConnectResponse{},
			takeoverErr: sentinel,
		}
		subc := &fakeSubscriptionCenter{}
		c := newTestClient(sc, subc, nil, noopPublishRoute)
		if _, err := c.AcquireSession(context.Background(), baseReq); !errors.Is(err, sentinel) {
			t.Fatalf("expected takeover error, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// SaveOfflineState
// ---------------------------------------------------------------------------

func TestSaveOfflineState(t *testing.T) {
	t.Run("validates owner", func(t *testing.T) {
		c := newTestClient(&fakeSessionCenter{}, &fakeSubscriptionCenter{}, nil, noopPublishRoute)
		err := c.SaveOfflineState(context.Background(), SaveOfflineStateRequest{ClientID: "c1"})
		if err == nil {
			t.Fatal("expected validation error for missing owner token")
		}
	})

	t.Run("passes fields through", func(t *testing.T) {
		sc := &fakeSessionCenter{}
		c := newTestClient(sc, &fakeSubscriptionCenter{}, nil, noopPublishRoute)
		req := SaveOfflineStateRequest{
			BrokerNodeID:          1,
			ClientID:              "c1",
			OwnerToken:            "tok",
			ClearWill:             true,
			SessionExpiryInterval: 120,
			NowUnixNano:           99,
		}
		if err := c.SaveOfflineState(context.Background(), req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if sc.saveReq == nil {
			t.Fatal("expected SaveOfflineState to be called")
		}
		if sc.saveReq.ClientID != "c1" || sc.saveReq.OwnerToken != "tok" || !sc.saveReq.ClearWill {
			t.Fatalf("fields not propagated: %+v", sc.saveReq)
		}
	})
}

// ---------------------------------------------------------------------------
// Subscribe
// ---------------------------------------------------------------------------

func TestSubscribe(t *testing.T) {
	t.Run("validates owner", func(t *testing.T) {
		c := newTestClient(&fakeSessionCenter{}, &fakeSubscriptionCenter{}, nil, noopPublishRoute)
		_, err := c.Subscribe(context.Background(), SubscribeRequest{ClientID: "c1", BrokerNodeID: 1})
		if err == nil {
			t.Fatal("expected validation error for missing owner token")
		}
	})

	t.Run("nil subscribe packet", func(t *testing.T) {
		c := newTestClient(&fakeSessionCenter{}, &fakeSubscriptionCenter{}, nil, noopPublishRoute)
		_, err := c.Subscribe(context.Background(), SubscribeRequest{ClientID: "c1", OwnerToken: "t", BrokerNodeID: 1})
		if err == nil {
			t.Fatal("expected error for nil subscribe packet")
		}
	})

	t.Run("caps qos and maps granted result", func(t *testing.T) {
		subc := &fakeSubscriptionCenter{
			createResp: &proto_topic.SubResponse{Topics: map[string]int32{"a": 1}},
		}
		c := newTestClient(&fakeSessionCenter{}, subc, nil, noopPublishRoute)
		resp, err := c.Subscribe(context.Background(), SubscribeRequest{
			ClientID:     "c1",
			OwnerToken:   "tok",
			BrokerNodeID: 1,
			MaxQoS:       1,
			Subscribe: &packets.Subscribe{
				Subscriptions: []packets.SubOptions{{Topic: "a", QoS: 2}},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.GrantedQoS) != 1 || resp.GrantedQoS[0] != 1 {
			t.Fatalf("unexpected granted QoS: %v", resp.GrantedQoS)
		}
		// QoS was capped to MaxQoS before reaching CreateSub.
		if len(subc.createReqs) != 1 {
			t.Fatalf("expected 1 CreateSub call, got %d", len(subc.createReqs))
		}
		if got := subc.createReqs[0].Topics[0].QoS; got != 1 {
			t.Fatalf("expected capped QoS 1 sent to CreateSub, got %d", got)
		}
	})

	t.Run("missing topic in response yields -1", func(t *testing.T) {
		subc := &fakeSubscriptionCenter{
			createResp: &proto_topic.SubResponse{Topics: map[string]int32{}},
		}
		c := newTestClient(&fakeSessionCenter{}, subc, nil, noopPublishRoute)
		resp, err := c.Subscribe(context.Background(), SubscribeRequest{
			ClientID:     "c1",
			OwnerToken:   "tok",
			BrokerNodeID: 1,
			Subscribe: &packets.Subscribe{
				Subscriptions: []packets.SubOptions{{Topic: "a", QoS: 1}},
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.GrantedQoS[0] != -1 {
			t.Fatalf("expected -1 for unacked topic, got %d", resp.GrantedQoS[0])
		}
	})
}

// ---------------------------------------------------------------------------
// DeleteClientSubscriptions
// ---------------------------------------------------------------------------

func TestDeleteClientSubscriptions(t *testing.T) {
	t.Run("validates owner", func(t *testing.T) {
		c := newTestClient(&fakeSessionCenter{}, &fakeSubscriptionCenter{}, nil, noopPublishRoute)
		err := c.DeleteClientSubscriptions(context.Background(), DeleteClientSubscriptionsRequest{ClientID: "c1", BrokerNodeID: 1})
		if err == nil {
			t.Fatal("expected validation error")
		}
	})

	t.Run("passes owner token through", func(t *testing.T) {
		subc := &fakeSubscriptionCenter{}
		c := newTestClient(&fakeSessionCenter{}, subc, nil, noopPublishRoute)
		err := c.DeleteClientSubscriptions(context.Background(), DeleteClientSubscriptionsRequest{
			ClientID: "c1", OwnerToken: "tok", BrokerNodeID: 1,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if subc.deleteReq == nil || subc.deleteReq.OwnerToken != "tok" {
			t.Fatalf("owner token not propagated: %+v", subc.deleteReq)
		}
	})
}

// ---------------------------------------------------------------------------
// ListClientSubscriptions
// ---------------------------------------------------------------------------

func TestListClientSubscriptions(t *testing.T) {
	t.Run("requires client id", func(t *testing.T) {
		c := newTestClient(&fakeSessionCenter{}, &fakeSubscriptionCenter{}, nil, noopPublishRoute)
		if _, err := c.ListClientSubscriptions(context.Background(), ListClientSubscriptionsRequest{BrokerNodeID: 1}); err == nil {
			t.Fatal("expected error for missing client id")
		}
	})

	t.Run("requires node id", func(t *testing.T) {
		c := newTestClient(&fakeSessionCenter{}, &fakeSubscriptionCenter{}, nil, noopPublishRoute)
		if _, err := c.ListClientSubscriptions(context.Background(), ListClientSubscriptionsRequest{ClientID: "c1"}); err == nil {
			t.Fatal("expected error for missing node id")
		}
	})

	t.Run("nil topics normalized to empty map", func(t *testing.T) {
		subc := &fakeSubscriptionCenter{listResp: &proto_topic.GetClientSubscriptionsResponse{Topics: nil}}
		c := newTestClient(&fakeSessionCenter{}, subc, nil, noopPublishRoute)
		resp, err := c.ListClientSubscriptions(context.Background(), ListClientSubscriptionsRequest{ClientID: "c1", BrokerNodeID: 1})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Topics == nil {
			t.Fatal("expected non-nil topics map")
		}
		if len(resp.Topics) != 0 {
			t.Fatalf("expected empty map, got %v", resp.Topics)
		}
	})

	t.Run("returns topics", func(t *testing.T) {
		subc := &fakeSubscriptionCenter{
			listResp: &proto_topic.GetClientSubscriptionsResponse{
				Topics: map[string]*proto_topic.SubOption{"a": {Topic: "a"}},
			},
		}
		c := newTestClient(&fakeSessionCenter{}, subc, nil, noopPublishRoute)
		resp, err := c.ListClientSubscriptions(context.Background(), ListClientSubscriptionsRequest{ClientID: "c1", BrokerNodeID: 1})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := resp.Topics["a"]; !ok {
			t.Fatalf("expected topic 'a' in result, got %v", resp.Topics)
		}
	})
}

// ---------------------------------------------------------------------------
// RoutePublish
// ---------------------------------------------------------------------------

func TestRoutePublish(t *testing.T) {
	validReq := RoutePublishRequest{
		BrokerNodeID: 1,
		ClientID:     "c1",
		OwnerToken:   "tok",
		Message:      &brokerpublish.Message{Publish: &packets.Publish{Topic: "a"}},
	}

	t.Run("validates owner", func(t *testing.T) {
		c := newTestClient(&fakeSessionCenter{}, &fakeSubscriptionCenter{}, nil, noopPublishRoute)
		err := c.RoutePublish(context.Background(), RoutePublishRequest{Message: validReq.Message})
		if err == nil {
			t.Fatal("expected validation error")
		}
	})

	t.Run("nil message", func(t *testing.T) {
		c := newTestClient(&fakeSessionCenter{}, &fakeSubscriptionCenter{}, nil, noopPublishRoute)
		err := c.RoutePublish(context.Background(), RoutePublishRequest{BrokerNodeID: 1, ClientID: "c1", OwnerToken: "tok"})
		if err == nil {
			t.Fatal("expected error for nil message")
		}
	})

	t.Run("invokes handler", func(t *testing.T) {
		called := false
		var gotReq RoutePublishRequest
		route := func(_ context.Context, r RoutePublishRequest) error {
			called = true
			gotReq = r
			return nil
		}
		c := newTestClient(&fakeSessionCenter{}, &fakeSubscriptionCenter{}, nil, route)
		if err := c.RoutePublish(context.Background(), validReq); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !called {
			t.Fatal("expected publish route handler to be invoked")
		}
		if gotReq.ClientID != "c1" {
			t.Fatalf("handler received wrong request: %+v", gotReq)
		}
	})

	t.Run("propagates handler error", func(t *testing.T) {
		sentinel := errors.New("route boom")
		route := func(context.Context, RoutePublishRequest) error { return sentinel }
		c := newTestClient(&fakeSessionCenter{}, &fakeSubscriptionCenter{}, nil, route)
		if err := c.RoutePublish(context.Background(), validReq); !errors.Is(err, sentinel) {
			t.Fatalf("expected handler error, got %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// ClosePreviousOwner
// ---------------------------------------------------------------------------

func TestClosePreviousOwner(t *testing.T) {
	validPrev := &proto_session.SessionOwner{ClientID: "c1", NodeID: 9, OwnerToken: "old-tok"}

	t.Run("forwards to node controller", func(t *testing.T) {
		nc := &fakeNodeController{}
		c := newTestClient(&fakeSessionCenter{}, &fakeSubscriptionCenter{}, nc, noopPublishRoute)
		err := c.ClosePreviousOwner(context.Background(), ClosePreviousOwnerRequest{
			BrokerNodeID:  1,
			ClientID:      "c1",
			PreviousOwner: validPrev,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !nc.called {
			t.Fatal("expected RequestCloseClient to be called")
		}
		if nc.nodeID != 9 || nc.clientID != "c1" || nc.ownerToken != "old-tok" {
			t.Fatalf("controller got wrong args: node=%d client=%s tok=%s", nc.nodeID, nc.clientID, nc.ownerToken)
		}
	})

	t.Run("nil node controller errors", func(t *testing.T) {
		c := newTestClient(&fakeSessionCenter{}, &fakeSubscriptionCenter{}, nil, noopPublishRoute)
		err := c.ClosePreviousOwner(context.Background(), ClosePreviousOwnerRequest{
			BrokerNodeID:  1,
			ClientID:      "c1",
			PreviousOwner: validPrev,
		})
		if err == nil {
			t.Fatal("expected error when node controller is nil")
		}
	})

	t.Run("validation", func(t *testing.T) {
		nc := &fakeNodeController{}
		c := newTestClient(&fakeSessionCenter{}, &fakeSubscriptionCenter{}, nc, noopPublishRoute)
		cases := []struct {
			name string
			req  ClosePreviousOwnerRequest
		}{
			{"missing client id", ClosePreviousOwnerRequest{BrokerNodeID: 1, PreviousOwner: validPrev}},
			{"missing node id", ClosePreviousOwnerRequest{ClientID: "c1", PreviousOwner: validPrev}},
			{"nil previous owner", ClosePreviousOwnerRequest{ClientID: "c1", BrokerNodeID: 1}},
			{"prev missing client id", ClosePreviousOwnerRequest{ClientID: "c1", BrokerNodeID: 1, PreviousOwner: &proto_session.SessionOwner{OwnerToken: "t"}}},
			{"prev missing token", ClosePreviousOwnerRequest{ClientID: "c1", BrokerNodeID: 1, PreviousOwner: &proto_session.SessionOwner{ClientID: "c1"}}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				if err := c.ClosePreviousOwner(context.Background(), tc.req); err == nil {
					t.Fatal("expected validation error")
				}
			})
		}
	})
}

// ---------------------------------------------------------------------------
// HasMatchingSubscribers (validation only; routing exercised elsewhere)
// ---------------------------------------------------------------------------

func TestHasMatchingSubscribersValidation(t *testing.T) {
	c := newTestClient(&fakeSessionCenter{}, &fakeSubscriptionCenter{}, nil, noopPublishRoute)
	cases := []struct {
		name string
		req  HasMatchingSubscribersRequest
	}{
		{"missing client id", HasMatchingSubscribersRequest{BrokerNodeID: 1, Topic: "a"}},
		{"missing node id", HasMatchingSubscribersRequest{ClientID: "c1", Topic: "a"}},
		{"missing topic", HasMatchingSubscribersRequest{ClientID: "c1", BrokerNodeID: 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := c.HasMatchingSubscribers(context.Background(), tc.req); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func assertCalls(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("call sequence mismatch:\n got=%v\nwant=%v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("call sequence mismatch at %d:\n got=%v\nwant=%v", i, got, want)
		}
	}
}
