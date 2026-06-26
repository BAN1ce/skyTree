package client

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/config"
	brokerplugin "github.com/BAN1ce/skyTree/internal/broker/plugin"
	"github.com/BAN1ce/skyTree/internal/broker/retain"
	"github.com/BAN1ce/skyTree/internal/broker/staterouter"
	submemory "github.com/BAN1ce/skyTree/internal/broker/subcenter/memory"
	"github.com/BAN1ce/skyTree/logger"
	clientcap "github.com/BAN1ce/skyTree/pkg/brokerapi/clientcap"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/subscription"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/wire"
	proto "github.com/BAN1ce/skyTree/proto/proto_topic"
)

func TestPubackPubrecReasonCodes_RetainAndQoSNotSupported(t *testing.T) {
	if packets.PubackRetainNotSupported != 0x9A {
		t.Errorf("PubackRetainNotSupported want 0x9A, got 0x%X", packets.PubackRetainNotSupported)
	}
	if packets.PubackQoSNotSupported != 0x9B {
		t.Errorf("PubackQoSNotSupported want 0x9B, got 0x%X", packets.PubackQoSNotSupported)
	}
	if packets.PubrecRetainNotSupported != 0x9A {
		t.Errorf("PubrecRetainNotSupported want 0x9A, got 0x%X", packets.PubrecRetainNotSupported)
	}
	if packets.PubrecQoSNotSupported != 0x9B {
		t.Errorf("PubrecQoSNotSupported want 0x9B, got 0x%X", packets.PubrecQoSNotSupported)
	}
}

func TestHandlePublish_NoMatchingSubscribersUsesConfiguredReason(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	cfg := mustLoadConfigForTest(t)
	oldNoSub := cfg.Broker.NoSubTopicResponse
	cfg.Broker.NoSubTopicResponse = packets.PubackNoMatchingSubscribers
	defer func() {
		cfg.Broker.NoSubTopicResponse = oldNoSub
	}()
	logger.LoadForTest()

	tests := []struct {
		name       string
		qos        byte
		packetType byte
	}{
		{name: "qos1", qos: 1, packetType: packets.PUBACK},
		{name: "qos2", qos: 2, packetType: packets.PUBREC},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c1, c2 := net.Pipe()
			defer c1.Close()
			defer c2.Close()

			cl := NewClient(c1,
				WithConfig(clientConfigForTest(cfg.Broker)),
				WithSubCenter(&recordingSubCenter{}),
				WithStateRouter(&testStateRouter{
					hasMatchingSubscribers: func(context.Context, staterouter.HasMatchingSubscribersRequest) (bool, error) {
						return false, nil
					},
					routePublish: func(context.Context, staterouter.RoutePublishRequest) error {
						return nil
					},
				}),
			)
			cl.ID = "no-sub-test"
			cl.ctx, cl.cancel = context.WithCancelCause(context.Background())

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

			handler := NewClientHandler(cl)
			if err := handler.handlePublish(context.Background(), &packets.Publish{
				Topic:    "a/no-subs",
				QoS:      tt.qos,
				PacketID: 77,
			}); err != nil {
				t.Fatalf("handlePublish: %v", err)
			}

			select {
			case cp := <-readCh:
				if cp.FixedHeader.Type != tt.packetType {
					t.Fatalf("expected packet type %d, got %d", tt.packetType, cp.FixedHeader.Type)
				}
				switch ack := cp.Content.(type) {
				case *packets.Puback:
					if ack.ReasonCode != packets.PubackNoMatchingSubscribers {
						t.Fatalf("expected No Matching Subscribers, got 0x%X", ack.ReasonCode)
					}
				case *packets.Pubrec:
					if ack.ReasonCode != packets.PubrecNoMatchingSubscribers {
						t.Fatalf("expected No Matching Subscribers, got 0x%X", ack.ReasonCode)
					}
				default:
					t.Fatalf("unexpected content %T", cp.Content)
				}
			case err := <-errCh:
				t.Fatalf("read packet error: %v", err)
			case <-time.After(3 * time.Second):
				t.Fatalf("timeout waiting for publish ack")
			}
		})
	}
}

func TestHandlePublish_NoMatchingSubscribersUsesDeliveryFilters(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	cfg := mustLoadConfigForTest(t)
	oldNoSub := cfg.Broker.NoSubTopicResponse
	cfg.Broker.NoSubTopicResponse = packets.PubackNoMatchingSubscribers
	defer func() {
		cfg.Broker.NoSubTopicResponse = oldNoSub
	}()
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	cl := NewClient(
		c1,
		WithConfig(clientConfigForTest(cfg.Broker)),
		WithSubCenter(&retainedMatchSubCenter{
			matches: []*proto.ClientMatch{
				{
					ClientID: "publisher",
					Matched: []*proto.MatchedSubscription{
						{TopicFilter: "a/#", QoS: 1, NoLocal: true},
					},
				},
			},
		}),
		WithStateRouter(&testStateRouter{
			hasMatchingSubscribers: func(context.Context, staterouter.HasMatchingSubscribersRequest) (bool, error) {
				return false, nil
			},
			routePublish: func(context.Context, staterouter.RoutePublishRequest) error {
				return nil
			},
		}),
	)
	cl.ID = "publisher"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	defer cl.cancel(nil)

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

	if err := NewClientHandler(cl).handlePublish(context.Background(), &packets.Publish{
		Topic:    "a/b",
		QoS:      1,
		PacketID: 78,
	}); err != nil {
		t.Fatalf("handlePublish: %v", err)
	}

	select {
	case cp := <-readCh:
		ack, ok := cp.Content.(*packets.Puback)
		if !ok {
			t.Fatalf("expected PUBACK, got %T", cp.Content)
		}
		if ack.ReasonCode != packets.PubackNoMatchingSubscribers {
			t.Fatalf("expected No Matching Subscribers after NoLocal filtering, got 0x%X", ack.ReasonCode)
		}
	case err := <-errCh:
		t.Fatalf("read packet error: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatalf("timeout waiting for PUBACK")
	}
}

func TestHandleSub_ReturnsWildcardNotSupportedWhenConfigDisablesWildcard(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Skipf("config init: %v (skip if no config)", err)
	}
	cfg := mustLoadConfigForTest(t)
	cfg.Broker.ConnectAckProperty.WildcardSubscriptionAvailable = false

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	cl := NewClient(c1, WithConfig(clientConfigForTest(cfg.Broker)))
	cl.ID = "cap-test"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	cl.connAckAccepted.Store(true)

	done := make(chan struct{})
	go func() {
		defer close(done)
		cp, err := wire.Decode(c2, wire.DecodeOptions{})
		if err != nil && err != io.EOF {
			t.Errorf("ReadPacket: %v", err)
			return
		}
		if cp == nil {
			return
		}
		if cp.FixedHeader.Type != packets.DISCONNECT {
			t.Errorf("expected DISCONNECT, got type %d", cp.FixedHeader.Type)
			return
		}
		disc, ok := cp.Content.(*packets.Disconnect)
		if !ok {
			t.Errorf("expected *Disconnect, got %T", cp.Content)
			return
		}
		if disc.ReasonCode != packets.DisconnectWildcardSubscriptionsNotSupported {
			t.Errorf("expected reason 0xA2 (Wildcard not supported), got 0x%X", disc.ReasonCode)
		}
	}()

	handler := NewClientHandler(cl)
	subscribe := &packets.Subscribe{
		PacketID: 1,
		Subscriptions: []packets.SubOptions{
			{Topic: "a/#", QoS: 1},
		},
	}
	if err := handler.handleSub(cl.ctx, subscribe); !errors.Is(err, ErrProtocolError) {
		t.Fatalf("expected protocol error, got %v", err)
	}

	c2.Close()
	<-done
}

func TestHandleSub_ReturnsSharedNotSupportedWhenConfigDisablesShared(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Skipf("config init: %v (skip if no config)", err)
	}
	cfg := mustLoadConfigForTest(t)
	cfg.Broker.ConnectAckProperty.SharedSubscriptionAvailable = false

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	cl := NewClient(c1, WithConfig(clientConfigForTest(cfg.Broker)))
	cl.ID = "cap-test"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	cl.connAckAccepted.Store(true)

	done := make(chan struct{})
	go func() {
		defer close(done)
		cp, err := wire.Decode(c2, wire.DecodeOptions{})
		if err != nil && err != io.EOF {
			t.Errorf("ReadPacket: %v", err)
			return
		}
		if cp == nil {
			return
		}
		disc, ok := cp.Content.(*packets.Disconnect)
		if !ok {
			return
		}
		if disc.ReasonCode != packets.DisconnectSharedSubscriptionNotSupported {
			t.Errorf("expected reason 0x9E (Shared not supported), got 0x%X", disc.ReasonCode)
		}
	}()

	handler := NewClientHandler(cl)
	subID := 1
	subscribe := &packets.Subscribe{
		PacketID: 1,
		Properties: &packets.SubscribeProperties{
			SubscriptionIdentifier: &subID,
		},
		Subscriptions: []packets.SubOptions{
			{Topic: "$share/group/topic", QoS: 1},
		},
	}
	if err := handler.handleSub(cl.ctx, subscribe); !errors.Is(err, ErrProtocolError) {
		t.Fatalf("expected protocol error, got %v", err)
	}

	c2.Close()
	<-done
}

func TestHandleSub_ReturnsSharedNotSupportedWhenRuntimeManagerMissing(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Skipf("config init: %v (skip if no config)", err)
	}
	cfg := mustLoadConfigForTest(t)
	oldSharedAvailable := cfg.Broker.ConnectAckProperty.SharedSubscriptionAvailable
	cfg.Broker.ConnectAckProperty.SharedSubscriptionAvailable = true
	defer func() {
		cfg.Broker.ConnectAckProperty.SharedSubscriptionAvailable = oldSharedAvailable
	}()
	logger.LoadForTest()

	center := submemory.NewMemorySubCenter()
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	cl := NewClient(c1, WithSubCenter(center))
	cl.ID = "cap-test"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	cl.connAckAccepted.Store(true)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
		cp, err := wire.Decode(c2, wire.DecodeOptions{})
		if err != nil {
			t.Errorf("ReadPacket: %v", err)
			return
		}
		disc, ok := cp.Content.(*packets.Disconnect)
		if !ok {
			t.Errorf("expected *Disconnect, got %T", cp.Content)
			return
		}
		if disc.ReasonCode != packets.DisconnectSharedSubscriptionNotSupported {
			t.Errorf("expected shared subscription not supported, got 0x%X", disc.ReasonCode)
		}
	}()

	handler := NewClientHandler(cl)
	subscribe := &packets.Subscribe{
		PacketID: 1,
		Subscriptions: []packets.SubOptions{
			{Topic: "$share/group/topic", QoS: 1},
		},
	}
	if err := handler.handleSub(cl.ctx, subscribe); !errors.Is(err, ErrProtocolError) {
		t.Fatalf("expected protocol error, got %v", err)
	}
	<-done

	subs, err := center.GetClientSubscriptions(context.Background(), &proto.GetClientSubscriptionsRequest{ClientID: cl.ID})
	if err != nil {
		t.Fatalf("GetClientSubscriptions: %v", err)
	}
	if got := subs.GetTopics()["$share/group/topic"]; got != nil {
		t.Fatalf("shared subscription should not be persisted when runtime manager is missing, got %+v", got)
	}
}

func TestHandleSub_ReturnsTopicFilterInvalidForMalformedSharedSubscription(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	center := submemory.NewMemorySubCenter()
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	cl := NewClient(c1, WithSubCenter(center))
	cl.ID = "cap-test"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
		cp, err := wire.Decode(c2, wire.DecodeOptions{})
		if err != nil {
			t.Errorf("ReadPacket: %v", err)
			return
		}
		suback, ok := cp.Content.(*packets.Suback)
		if !ok {
			t.Errorf("expected *Suback, got %T", cp.Content)
			return
		}
		if len(suback.Reasons) != 1 || suback.Reasons[0] != packets.SubackTopicFilterinvalid {
			t.Errorf("expected topic filter invalid, got %+v", suback.Reasons)
		}
	}()

	handler := NewClientHandler(cl)
	subscribe := &packets.Subscribe{
		PacketID: 1,
		Subscriptions: []packets.SubOptions{
			{Topic: "$share/group", QoS: 1},
		},
	}
	if err := handler.handleSub(cl.ctx, subscribe); err != nil {
		t.Fatalf("handleSub should write SUBACK instead of returning error: %v", err)
	}
	<-done

	subs, err := center.GetClientSubscriptions(context.Background(), &proto.GetClientSubscriptionsRequest{ClientID: cl.ID})
	if err != nil {
		t.Fatalf("GetClientSubscriptions: %v", err)
	}
	if len(subs.GetTopics()) != 0 {
		t.Fatalf("malformed shared subscription should not be persisted, got %+v", subs.GetTopics())
	}
}

func TestHandleSub_ReturnsTopicFilterInvalidForSharedGroupWildcard(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	center := submemory.NewMemorySubCenter()
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	cl := NewClient(c1, WithSubCenter(center))
	cl.ID = "cap-test-share-group-wildcard"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
		cp, err := wire.Decode(c2, wire.DecodeOptions{})
		if err != nil {
			t.Errorf("ReadPacket: %v", err)
			return
		}
		suback, ok := cp.Content.(*packets.Suback)
		if !ok {
			t.Errorf("expected *Suback, got %T", cp.Content)
			return
		}
		if len(suback.Reasons) != 1 || suback.Reasons[0] != packets.SubackTopicFilterinvalid {
			t.Errorf("expected topic filter invalid, got %+v", suback.Reasons)
		}
	}()

	subscribe := &packets.Subscribe{
		PacketID: 2,
		Subscriptions: []packets.SubOptions{
			{Topic: "$share/g+/topic", QoS: 1},
		},
	}
	if err := NewClientHandler(cl).handleSub(cl.ctx, subscribe); err != nil {
		t.Fatalf("handleSub should write SUBACK instead of returning error: %v", err)
	}
	<-done

	subs, err := center.GetClientSubscriptions(context.Background(), &proto.GetClientSubscriptionsRequest{ClientID: cl.ID})
	if err != nil {
		t.Fatalf("GetClientSubscriptions: %v", err)
	}
	if len(subs.GetTopics()) != 0 {
		t.Fatalf("malformed shared subscription should not be persisted, got %+v", subs.GetTopics())
	}
}

func TestHandleSub_DisconnectsForSharedNoLocal(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	center := submemory.NewMemorySubCenter()
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	cl := NewClient(c1, WithSubCenter(center))
	cl.ID = "cap-test"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	cl.connAckAccepted.Store(true)

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

	handler := NewClientHandler(cl)
	subscribe := &packets.Subscribe{
		PacketID: 1,
		Subscriptions: []packets.SubOptions{
			{Topic: "$share/group/topic", QoS: 1, NoLocal: true},
		},
	}
	if err := handler.handleSub(cl.ctx, subscribe); !errors.Is(err, ErrProtocolError) {
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

	subs, err := center.GetClientSubscriptions(context.Background(), &proto.GetClientSubscriptionsRequest{ClientID: cl.ID})
	if err != nil {
		t.Fatalf("GetClientSubscriptions: %v", err)
	}
	if len(subs.GetTopics()) != 0 {
		t.Fatalf("shared NoLocal subscription should not be persisted, got %+v", subs.GetTopics())
	}
}

func TestHandleSub_ReturnsSubscriptionIdentifierNotSupportedWhenConfigDisablesIt(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Skipf("config init: %v (skip if no config)", err)
	}
	cfg := mustLoadConfigForTest(t)
	cfg.Broker.ConnectAckProperty.SubscriptionIdentifierAvailable = false

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	cl := NewClient(c1, WithConfig(clientConfigForTest(cfg.Broker)))
	cl.ID = "cap-test"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	cl.connAckAccepted.Store(true)

	done := make(chan struct{})
	go func() {
		defer close(done)
		cp, err := wire.Decode(c2, wire.DecodeOptions{})
		if err != nil && err != io.EOF {
			t.Errorf("ReadPacket: %v", err)
			return
		}
		if cp == nil {
			return
		}
		disc, ok := cp.Content.(*packets.Disconnect)
		if !ok {
			return
		}
		if disc.ReasonCode != packets.DisconnectSubscriptionIdentifiersNotSupported {
			t.Errorf("expected reason 0xA1 (Subscription Identifier not supported), got 0x%X", disc.ReasonCode)
		}
	}()

	handler := NewClientHandler(cl)
	subID := 1
	subscribe := &packets.Subscribe{
		PacketID: 1,
		Properties: &packets.SubscribeProperties{
			SubscriptionIdentifier: &subID,
		},
		Subscriptions: []packets.SubOptions{
			{Topic: "plain/topic", QoS: 1},
		},
	}
	if err := handler.handleSub(cl.ctx, subscribe); !errors.Is(err, ErrProtocolError) {
		t.Fatalf("expected protocol error, got %v", err)
	}

	c2.Close()
	<-done
}

func TestHandleSub_ReturnsTopicFilterInvalidForMalformedFilter(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	cl := NewClient(c1, WithSubCenter(submemory.NewMemorySubCenter()))
	cl.ID = "cap-test"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
		cp, err := wire.Decode(c2, wire.DecodeOptions{})
		if err != nil {
			t.Errorf("ReadPacket: %v", err)
			return
		}
		suback, ok := cp.Content.(*packets.Suback)
		if !ok {
			t.Errorf("expected *Suback, got %T", cp.Content)
			return
		}
		if len(suback.Reasons) != 1 {
			t.Errorf("expected 1 reason, got %d", len(suback.Reasons))
			return
		}
		if suback.Reasons[0] != packets.SubackTopicFilterinvalid {
			t.Errorf("expected topic filter invalid, got 0x%X", suback.Reasons[0])
		}
	}()

	handler := NewClientHandler(cl)
	subscribe := &packets.Subscribe{
		PacketID: 1,
		Subscriptions: []packets.SubOptions{
			{Topic: "a/#/b", QoS: 1},
		},
	}
	if err := handler.handleSub(cl.ctx, subscribe); err != nil {
		t.Fatalf("handleSub should write SUBACK instead of returning error: %v", err)
	}

	<-done
}

func TestHandleUnsub_ReturnsTopicFilterInvalidForMalformedFilter(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	center := submemory.NewMemorySubCenter()
	if _, err := center.CreateSub(context.Background(), &proto.SubRequest{
		ClientID: "cap-test",
		Topics: []*proto.SubOption{
			{Topic: "valid/topic", QoS: 1},
		},
	}); err != nil {
		t.Fatalf("CreateSub: %v", err)
	}

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	cl := NewClient(c1, WithSubCenter(center))
	cl.ID = "cap-test"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
		cp, err := wire.Decode(c2, wire.DecodeOptions{})
		if err != nil {
			t.Errorf("ReadPacket: %v", err)
			return
		}
		unsuback, ok := cp.Content.(*packets.Unsuback)
		if !ok {
			t.Errorf("expected *Unsuback, got %T", cp.Content)
			return
		}
		if len(unsuback.Reasons) != 2 {
			t.Errorf("expected 2 reasons, got %d", len(unsuback.Reasons))
			return
		}
		if unsuback.Reasons[0] != packets.UnsubackTopicFilterInvalid {
			t.Errorf("expected malformed filter reason 0x8F, got 0x%X", unsuback.Reasons[0])
		}
		if unsuback.Reasons[1] != packets.UnsubackSuccess {
			t.Errorf("expected valid unsubscribe success, got 0x%X", unsuback.Reasons[1])
		}
	}()

	handler := NewClientHandler(cl)
	if err := handler.handleUnsub(cl.ctx, &packets.Unsubscribe{
		PacketID: 1,
		Topics:   []string{"a/#/b", "valid/topic"},
	}); err != nil {
		t.Fatalf("handleUnsub should write UNSUBACK instead of returning error: %v", err)
	}

	<-done
}

func TestHandleUnsub_ReturnsTopicFilterInvalidForMalformedSharedFilter(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	cl := NewClient(c1, WithSubCenter(submemory.NewMemorySubCenter()))
	cl.ID = "cap-test"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
		cp, err := wire.Decode(c2, wire.DecodeOptions{})
		if err != nil {
			t.Errorf("ReadPacket: %v", err)
			return
		}
		unsuback, ok := cp.Content.(*packets.Unsuback)
		if !ok {
			t.Errorf("expected *Unsuback, got %T", cp.Content)
			return
		}
		if len(unsuback.Reasons) != 1 || unsuback.Reasons[0] != packets.UnsubackTopicFilterInvalid {
			t.Errorf("expected shared topic filter invalid, got %+v", unsuback.Reasons)
		}
	}()

	handler := NewClientHandler(cl)
	if err := handler.handleUnsub(cl.ctx, &packets.Unsubscribe{
		PacketID: 1,
		Topics:   []string{"$share/group/a/#/b"},
	}); err != nil {
		t.Fatalf("handleUnsub should write UNSUBACK instead of returning error: %v", err)
	}

	<-done
}

func TestHandleSub_PersistsGrantedQoSCappedByMaximumQoS(t *testing.T) {
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

	center := submemory.NewMemorySubCenter()
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	cl := NewClient(
		c1,
		WithConfig(clientConfigForTest(cfg.Broker)),
		WithSubCenter(center),
		WithStateRouter(&testStateRouter{
			subscribe: func(ctx context.Context, req staterouter.SubscribeRequest) (*staterouter.SubscribeResponse, error) {
				subscribePacket := *req.Subscribe
				subscribePacket.Subscriptions = append([]packets.SubOptions(nil), req.Subscribe.Subscriptions...)
				for idx := range subscribePacket.Subscriptions {
					if int(subscribePacket.Subscriptions[idx].QoS) > req.MaxQoS {
						subscribePacket.Subscriptions[idx].QoS = byte(req.MaxQoS)
					}
				}
				rsp, err := center.CreateSub(ctx, subscription.NewProtoSubRequest(&subscribePacket, req.ClientID, req.OwnerToken))
				if err != nil {
					return nil, err
				}
				granted := make([]int32, 0, len(subscribePacket.Subscriptions))
				for _, sub := range subscribePacket.Subscriptions {
					granted = append(granted, rsp.GetTopics()[sub.Topic])
				}
				return &staterouter.SubscribeResponse{GrantedQoS: granted}, nil
			},
		}),
	)
	cl.ID = "cap-test"
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())

	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
		cp, err := wire.Decode(c2, wire.DecodeOptions{})
		if err != nil {
			t.Errorf("ReadPacket: %v", err)
			return
		}
		suback, ok := cp.Content.(*packets.Suback)
		if !ok {
			t.Errorf("expected *Suback, got %T", cp.Content)
			return
		}
		if len(suback.Reasons) != 1 || suback.Reasons[0] != packets.SubackGrantedQoS1 {
			t.Errorf("expected granted QoS1, got %+v", suback.Reasons)
		}
	}()

	handler := NewClientHandler(cl)
	subscribe := &packets.Subscribe{
		PacketID: 1,
		Subscriptions: []packets.SubOptions{
			{Topic: "a/b", QoS: 2},
		},
	}
	if err := handler.handleSub(cl.ctx, subscribe); err != nil {
		t.Fatalf("handleSub: %v", err)
	}
	<-readDone

	subs, err := center.GetClientSubscriptions(context.Background(), &proto.GetClientSubscriptionsRequest{ClientID: cl.ID})
	if err != nil {
		t.Fatalf("GetClientSubscriptions: %v", err)
	}
	got := subs.GetTopics()["a/b"].GetQoS()
	if got != 1 {
		t.Fatalf("expected persisted QoS to be capped to 1, got %d", got)
	}
}

// MQTT5 §3.1.2.11.7：失败 CONNACK 必须能携带 ReasonString，便于客户端排障。
func TestValidConnectPopulatesReasonStringOnFailure(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

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

	cl := NewClient(c1)
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())
	err := NewClientHandler(cl).validConnect(&packets.Connect{
		ProtocolName:    "MQTT",
		ProtocolVersion: 0x04, // 不支持的版本
		ClientID:        "client-bad-version",
	})
	if err == nil {
		t.Fatal("expected validConnect to fail")
	}

	select {
	case cp := <-readCh:
		conAck, ok := cp.Content.(*packets.ConnAck)
		if !ok {
			t.Fatalf("expected CONNACK, got %T", cp.Content)
		}
		if conAck.ReasonCode != packets.ConnAckUnsupportedProtocolVersion {
			t.Fatalf("expected unsupported protocol reason, got 0x%X", conAck.ReasonCode)
		}
		if conAck.Properties == nil || conAck.Properties.ReasonString == "" {
			t.Fatal("expected ReasonString to be populated for failed CONNACK")
		}
	case err := <-errCh:
		t.Fatalf("read error: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for failed CONNACK")
	}
}

func TestValidConnectAllowsEmptyWillPayload(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	go func() {
		_ = c2.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _ = wire.Decode(c2, wire.DecodeOptions{})
	}()

	cl := NewClient(c1)
	cl.ctx, cl.cancel = context.WithCancelCause(context.Background())

	err := NewClientHandler(cl).validConnect(&packets.Connect{
		ProtocolName:    "MQTT",
		ProtocolVersion: 5,
		ClientID:        "will-empty",
		CleanStart:      true,
		WillFlag:        true,
		WillTopic:       "will/topic",
		WillMessage:     []byte{},
	})
	if err != nil {
		t.Fatalf("expected empty will payload to be valid, got %v", err)
	}
}

func TestSubackReasonCodeConstants(t *testing.T) {
	if packets.SubackWildcardsubscriptionsnotsupported != 0xA2 {
		t.Errorf("SubackWildcardsubscriptionsnotsupported want 0xA2, got 0x%X", packets.SubackWildcardsubscriptionsnotsupported)
	}
	if packets.SubackSharedSubscriptionnotsupported != 0x9E {
		t.Errorf("SubackSharedSubscriptionnotsupported want 0x9E, got 0x%X", packets.SubackSharedSubscriptionnotsupported)
	}
	if packets.SubackSubscriptionIdentifiersnotsupported != 0xA1 {
		t.Errorf("SubackSubscriptionIdentifiersnotsupported want 0xA1, got 0x%X", packets.SubackSubscriptionIdentifiersnotsupported)
	}
}

func TestPubackReasonStrings_RetainAndQoSNotSupported(t *testing.T) {
	puback := &packets.Puback{ReasonCode: packets.PubackRetainNotSupported}
	if s := puback.Reason(); s == "" {
		t.Error("expected non-empty Reason() for PubackRetainNotSupported")
	}
	puback.ReasonCode = packets.PubackQoSNotSupported
	if s := puback.Reason(); s == "" {
		t.Error("expected non-empty Reason() for PubackQoSNotSupported")
	}
}

func TestPubrecReasonStrings_RetainAndQoSNotSupported(t *testing.T) {
	pubrec := &packets.Pubrec{ReasonCode: packets.PubrecRetainNotSupported}
	if s := pubrec.Reason(); s == "" {
		t.Error("expected non-empty Reason() for PubrecRetainNotSupported")
	}
	pubrec.ReasonCode = packets.PubrecQoSNotSupported
	if s := pubrec.Reason(); s == "" {
		t.Error("expected non-empty Reason() for PubrecQoSNotSupported")
	}
}

func TestConnectAckProperty_AsUpperLimit(t *testing.T) {
	// Document that config fields are the upper limit; values at or below are allowed.
	prop := config.ConnectAckProperty{
		MaxQos:                          2,
		RetainAvailable:                 1,
		WildcardSubscriptionAvailable:   true,
		SharedSubscriptionAvailable:     true,
		SubscriptionIdentifierAvailable: true,
	}
	if prop.MaxQos != 2 {
		t.Fatal("MaxQos 2 means client can use 0,1,2")
	}
	if prop.RetainAvailable != 1 {
		t.Fatal("RetainAvailable 1 means retain allowed")
	}
	if !prop.WildcardSubscriptionAvailable {
		t.Fatal("WildcardSubscriptionAvailable true means #/+ allowed")
	}
}

func TestHandlePublish_DisconnectsOnInvalidTopicName(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}

	tests := []struct {
		name  string
		topic string
	}{
		{name: "empty topic without alias", topic: ""},
		{name: "wildcard plus", topic: "a/+"},
		{name: "wildcard hash", topic: "a/#"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp := runPublishExpectDisconnect(t, &packets.Publish{
				Topic:      tt.topic,
				Properties: &packets.PublishProperties{},
			})

			disc := cp.Content.(*packets.Disconnect)
			if disc.ReasonCode != packets.DisconnectTopicNameInvalid {
				t.Fatalf("expected Topic Name Invalid, got 0x%X", disc.ReasonCode)
			}
		})
	}
}

func TestHandlePublish_DisconnectsWhenClientSendsSubscriptionIdentifier(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}

	cp := runPublishExpectDisconnect(t, &packets.Publish{
		Topic: "a/b",
		Properties: &packets.PublishProperties{
			SubscriptionIdentifier: []int{1},
		},
	})

	disc := cp.Content.(*packets.Disconnect)
	if disc.ReasonCode != packets.DisconnectProtocolError {
		t.Fatalf("expected Protocol Error, got 0x%X", disc.ReasonCode)
	}
	if disc.Properties == nil || !strings.Contains(disc.Properties.ReasonString, "Subscription Identifier") {
		t.Fatalf("expected reason string to mention Subscription Identifier, got %+v", disc.Properties)
	}
}

func TestHandlePublishRejectsInvalidUTF8WhenPayloadFormatIsText(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	payloadFormatText := byte(1)

	tests := []struct {
		name       string
		publish    *packets.Publish
		packetType byte
		reasonCode byte
	}{
		{
			name: "qos0 disconnect",
			publish: &packets.Publish{
				Topic:      "a/b",
				QoS:        0,
				Payload:    []byte{0xff, 0xfe},
				Properties: &packets.PublishProperties{PayloadFormat: &payloadFormatText},
			},
			packetType: packets.DISCONNECT,
			reasonCode: packets.DisconnectPayloadFormatInvalid,
		},
		{
			name: "qos1 puback",
			publish: &packets.Publish{
				Topic:      "a/b",
				QoS:        1,
				PacketID:   7,
				Payload:    []byte{0xff, 0xfe},
				Properties: &packets.PublishProperties{PayloadFormat: &payloadFormatText},
			},
			packetType: packets.PUBACK,
			reasonCode: packets.PubackPayloadFormatInvalid,
		},
		{
			name: "qos2 pubrec",
			publish: &packets.Publish{
				Topic:      "a/b",
				QoS:        2,
				PacketID:   8,
				Payload:    []byte{0xff, 0xfe},
				Properties: &packets.PublishProperties{PayloadFormat: &payloadFormatText},
			},
			packetType: packets.PUBREC,
			reasonCode: packets.PubrecPayloadFormatInvalid,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp := runPublishExpectPacket(t, tt.publish)
			if cp.FixedHeader.Type != tt.packetType {
				t.Fatalf("expected packet type %d, got %d", tt.packetType, cp.FixedHeader.Type)
			}
			switch p := cp.Content.(type) {
			case *packets.Disconnect:
				if p.ReasonCode != tt.reasonCode {
					t.Fatalf("expected reason 0x%X, got 0x%X", tt.reasonCode, p.ReasonCode)
				}
			case *packets.Puback:
				if p.ReasonCode != tt.reasonCode {
					t.Fatalf("expected reason 0x%X, got 0x%X", tt.reasonCode, p.ReasonCode)
				}
			case *packets.Pubrec:
				if p.ReasonCode != tt.reasonCode {
					t.Fatalf("expected reason 0x%X, got 0x%X", tt.reasonCode, p.ReasonCode)
				}
			default:
				t.Fatalf("unexpected content %T", cp.Content)
			}
		})
	}
}

func TestHandlePublish_DisconnectsQoS0RetainWhenRetainUnavailable(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	cfg := mustLoadConfigForTest(t)
	oldRetainAvailable := cfg.Broker.ConnectAckProperty.RetainAvailable
	cfg.Broker.ConnectAckProperty.RetainAvailable = 0
	defer func() {
		cfg.Broker.ConnectAckProperty.RetainAvailable = oldRetainAvailable
	}()

	cp := runPublishExpectDisconnectWithBrokerConfig(t, cfg.Broker, &packets.Publish{
		Topic:      "a/b",
		Retain:     true,
		Properties: &packets.PublishProperties{},
	})

	disc := cp.Content.(*packets.Disconnect)
	if disc.ReasonCode != packets.DisconnectRetainNotSupported {
		t.Fatalf("expected Retain Not Supported, got 0x%X", disc.ReasonCode)
	}
}

func TestHandlePublish_DisconnectsQoS1QoS2RetainWhenRetainUnavailable(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	cfg := mustLoadConfigForTest(t)
	oldRetainAvailable := cfg.Broker.ConnectAckProperty.RetainAvailable
	cfg.Broker.ConnectAckProperty.RetainAvailable = 0
	defer func() {
		cfg.Broker.ConnectAckProperty.RetainAvailable = oldRetainAvailable
	}()

	tests := []struct {
		name string
		qos  byte
	}{
		{name: "qos1", qos: 1},
		{name: "qos2", qos: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp := runPublishExpectDisconnectWithBrokerConfig(t, cfg.Broker, &packets.Publish{
				Topic:      "a/b",
				QoS:        tt.qos,
				PacketID:   10,
				Retain:     true,
				Properties: &packets.PublishProperties{},
			})

			disc := cp.Content.(*packets.Disconnect)
			if disc.ReasonCode != packets.DisconnectRetainNotSupported {
				t.Fatalf("expected Retain Not Supported, got 0x%X", disc.ReasonCode)
			}
		})
	}
}

func TestHandlePublish_DisconnectsWhenPublishExceedsMaximumQoS(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	cfg := mustLoadConfigForTest(t)
	oldMaxQoS := cfg.Broker.ConnectAckProperty.MaxQos
	cfg.Broker.ConnectAckProperty.MaxQos = 0
	defer func() {
		cfg.Broker.ConnectAckProperty.MaxQos = oldMaxQoS
	}()

	tests := []struct {
		name string
		qos  byte
	}{
		{name: "qos1", qos: 1},
		{name: "qos2", qos: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp := runPublishExpectDisconnectWithBrokerConfig(t, cfg.Broker, &packets.Publish{
				Topic:      "a/b",
				QoS:        tt.qos,
				PacketID:   11,
				Properties: &packets.PublishProperties{},
			})

			disc := cp.Content.(*packets.Disconnect)
			if disc.ReasonCode != packets.DisconnectQoSNotSupported {
				t.Fatalf("expected QoS Not Supported, got 0x%X", disc.ReasonCode)
			}
		})
	}
}

func TestHandlePubRelSendsPacketIdentifierNotFoundWhenStateMissing(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	c := NewClient(c1)
	c.ID = "qos2-test"
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

	err := NewClientHandler(c).handlePubRel(context.Background(), &packets.Pubrel{PacketID: 77})
	if err != nil {
		t.Fatalf("missing PUBREL state should be answered with PUBCOMP, not returned as handler error: %v", err)
	}

	select {
	case cp := <-readCh:
		pubcomp, ok := cp.Content.(*packets.Pubcomp)
		if !ok {
			t.Fatalf("expected PUBCOMP, got %T", cp.Content)
		}
		if pubcomp.PacketID != 77 {
			t.Fatalf("expected packet id 77, got %d", pubcomp.PacketID)
		}
		if pubcomp.ReasonCode != packets.PubcompPacketIdentifierNotFound {
			t.Fatalf("expected Packet Identifier Not Found, got 0x%X", pubcomp.ReasonCode)
		}
	case err := <-errCh:
		t.Fatalf("read packet error: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for PUBCOMP")
	}
}

func TestHandlePublishClearsWireFlagsForLiveDelivery(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	var delivered *packets.Publish
	c := NewClient(
		c1,
		WithRetain(retain.NewRetainStore(newClientRetainMemHashStore())),
		WithStateRouter(newTestPublishStateRouter(func(_ context.Context, msg *brokerpublish.Message) error {
			delivered = msg.GetPublish()
			return nil
		})),
	)
	c.ID = "live-flags-test"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())

	err := NewClientHandler(c).handlePublish(context.Background(), &packets.Publish{
		Topic:      "live/retain",
		Duplicate:  true,
		QoS:        0,
		Retain:     true,
		Payload:    []byte("value"),
		Properties: &packets.PublishProperties{},
	})
	if err != nil {
		t.Fatalf("handle publish: %v", err)
	}
	if delivered == nil {
		t.Fatal("expected live publish to be delivered")
	}
	if delivered.Duplicate {
		t.Fatal("live delivery must not propagate inbound DUP flag")
	}
	if delivered.Retain {
		t.Fatal("live delivery must not propagate inbound RETAIN flag")
	}
}

func TestHandlePublishDisconnectsWhenIncomingMessageRateTooHigh(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	delivered := 0
	c := NewClient(c1, WithStateRouter(newTestPublishStateRouter(func(context.Context, *brokerpublish.Message) error {
		delivered++
		return nil
	})))
	c.ID = "rate-limited-client"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.connAckAccepted.Store(true)
	c.incomingPublishRateLimiter = newIncomingPublishRateLimiter(1, time.Second)

	handler := NewClientHandler(c)
	if err := handler.handlePublish(context.Background(), &packets.Publish{
		Topic:      "rate/topic",
		Payload:    []byte("first"),
		Properties: &packets.PublishProperties{},
	}); err != nil {
		t.Fatalf("first publish should pass: %v", err)
	}

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

	err := handler.handlePublish(context.Background(), &packets.Publish{
		Topic:      "rate/topic",
		Payload:    []byte("second"),
		Properties: &packets.PublishProperties{},
	})
	if err != ErrProtocolError {
		t.Fatalf("expected protocol error on rate limit, got %v", err)
	}
	if delivered != 1 {
		t.Fatalf("rate limited publish must not be delivered, delivered=%d", delivered)
	}

	select {
	case cp := <-readCh:
		disc, ok := cp.Content.(*packets.Disconnect)
		if !ok {
			t.Fatalf("expected DISCONNECT, got %T", cp.Content)
		}
		if disc.ReasonCode != packets.DisconnectMessageRateTooHigh {
			t.Fatalf("expected Message Rate Too High, got 0x%X", disc.ReasonCode)
		}
	case err := <-errCh:
		t.Fatalf("read disconnect: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for DISCONNECT")
	}
}

func TestHandlePubRelWithNegativeReasonDoesNotDeliverQoS2Publish(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	delivered := false
	c := NewClient(c1, WithStateRouter(newTestPublishStateRouter(func(context.Context, *brokerpublish.Message) error {
		delivered = true
		return nil
	})))
	c.ID = "qos2-negative-pubrel"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())

	msg := &brokerpublish.Message{}
	msg.SetPublish(&packets.Publish{
		Topic:      "qos2/topic",
		QoS:        2,
		PacketID:   88,
		Payload:    []byte("value"),
		Properties: &packets.PublishProperties{},
	})
	c.QoS2.Store(msg)

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

	err := NewClientHandler(c).handlePubRel(context.Background(), &packets.Pubrel{
		PacketID:   88,
		ReasonCode: packets.PubrelPacketIdentifierNotFound,
	})
	if err != nil {
		t.Fatalf("negative PUBREL should complete without handler error: %v", err)
	}
	if delivered {
		t.Fatal("negative PUBREL must not release the stored QoS2 publish")
	}

	select {
	case cp := <-readCh:
		pubcomp, ok := cp.Content.(*packets.Pubcomp)
		if !ok {
			t.Fatalf("expected PUBCOMP, got %T", cp.Content)
		}
		if pubcomp.PacketID != 88 {
			t.Fatalf("expected packet id 88, got %d", pubcomp.PacketID)
		}
	case err := <-errCh:
		t.Fatalf("read packet error: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for PUBCOMP")
	}
}

func TestHandlePubRelKeepsQoS2StateWhenDeliveryFails(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	publishErr := errors.New("publish store unavailable")
	c := NewClient(c1, WithStateRouter(newTestPublishStateRouter(func(context.Context, *brokerpublish.Message) error {
		return publishErr
	})))
	c.ID = "qos2-pubrel-fail"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())

	msg := &brokerpublish.Message{}
	msg.SetPublish(&packets.Publish{
		Topic:      "qos2/topic",
		QoS:        2,
		PacketID:   89,
		Payload:    []byte("value"),
		Properties: &packets.PublishProperties{},
	})
	c.QoS2.Store(msg)

	readCh := make(chan *packets.ControlPacket, 1)
	errCh := make(chan error, 1)
	go func() {
		_ = c2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		cp, err := wire.Decode(c2, wire.DecodeOptions{})
		if err != nil {
			errCh <- err
			return
		}
		readCh <- cp
	}()

	err := NewClientHandler(c).handlePubRel(context.Background(), &packets.Pubrel{PacketID: 89})
	if !errors.Is(err, publishErr) {
		t.Fatalf("expected publish error, got %v", err)
	}
	if _, ok := c.QoS2.Read(89); !ok {
		t.Fatal("qos2 state must remain when delivery fails")
	}

	select {
	case cp := <-readCh:
		if pubcomp, ok := cp.Content.(*packets.Pubcomp); ok && pubcomp.ReasonCode == 0 {
			t.Fatalf("must not send success PUBCOMP after delivery failure: %+v", pubcomp)
		}
	case <-errCh:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for PUBCOMP read result")
	}
}

func TestHandlePubRelCompletesNonRetainedQoS2Publish(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	delivered := 0
	c := NewClient(c1, WithStateRouter(newTestPublishStateRouter(func(context.Context, *brokerpublish.Message) error {
		delivered++
		return nil
	})))
	c.ID = "qos2-pubrel-complete"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())

	msg := &brokerpublish.Message{}
	msg.SetPublish(&packets.Publish{
		Topic:      "qos2/non-retained",
		QoS:        2,
		PacketID:   90,
		Payload:    []byte("value"),
		Properties: &packets.PublishProperties{},
	})
	c.QoS2.Store(msg)

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

	if err := NewClientHandler(c).handlePubRel(context.Background(), &packets.Pubrel{PacketID: 90}); err != nil {
		t.Fatalf("handle PUBREL: %v", err)
	}
	if delivered != 1 {
		t.Fatalf("expected QoS2 publish delivered once, got %d", delivered)
	}
	if _, ok := c.QoS2.Read(90); ok {
		t.Fatal("qos2 state must be deleted after successful non-retained PUBREL")
	}

	select {
	case cp := <-readCh:
		pubcomp, ok := cp.Content.(*packets.Pubcomp)
		if !ok {
			t.Fatalf("expected PUBCOMP, got %T", cp.Content)
		}
		if pubcomp.PacketID != 90 || pubcomp.ReasonCode != packets.PubcompSuccess {
			t.Fatalf("unexpected PUBCOMP: %+v", pubcomp)
		}
	case err := <-errCh:
		t.Fatalf("read packet error: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for PUBCOMP")
	}
}

func TestHandlePubRelSkipsLiveDeliveryWhenStoredPubrecIsNoMatchingSubscribers(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	delivered := 0
	c := NewClient(c1, WithStateRouter(newTestPublishStateRouter(func(context.Context, *brokerpublish.Message) error {
		delivered++
		return nil
	})))
	c.ID = "qos2-pubrel-no-match"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())

	msg := &brokerpublish.Message{AckReasonCode: packets.PubrecNoMatchingSubscribers}
	msg.SetPublish(&packets.Publish{
		Topic:      "qos2/no-matching",
		QoS:        2,
		PacketID:   190,
		Payload:    []byte("value"),
		Properties: &packets.PublishProperties{},
	})
	c.QoS2.Store(msg)

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

	if err := NewClientHandler(c).handlePubRel(context.Background(), &packets.Pubrel{PacketID: 190}); err != nil {
		t.Fatalf("handle PUBREL: %v", err)
	}
	if delivered != 0 {
		t.Fatalf("expected no live delivery when PUBREC reason is no-matching, got %d", delivered)
	}
	if _, ok := c.QoS2.Read(190); ok {
		t.Fatal("qos2 state must be deleted after terminal PUBREL handling")
	}

	select {
	case cp := <-readCh:
		pubcomp, ok := cp.Content.(*packets.Pubcomp)
		if !ok {
			t.Fatalf("expected PUBCOMP, got %T", cp.Content)
		}
		if pubcomp.PacketID != 190 || pubcomp.ReasonCode != packets.PubcompSuccess {
			t.Fatalf("unexpected PUBCOMP: %+v", pubcomp)
		}
	case err := <-errCh:
		t.Fatalf("read packet error: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for PUBCOMP")
	}
}

func TestHandlePublishDuplicateQoS2ReusesStoredPubrecReasonBeforePlugin(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	cfg := mustLoadConfigForTest(t)
	oldNoSub := cfg.Broker.NoSubTopicResponse
	cfg.Broker.NoSubTopicResponse = packets.PubackNoMatchingSubscribers
	defer func() {
		cfg.Broker.NoSubTopicResponse = oldNoSub
	}()
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	pluginCalled := false
	c := NewClient(c1,
		WithConfig(clientConfigForTest(cfg.Broker)),
		WithSubCenter(&recordingSubCenter{}),
		WithStateRouter(&testStateRouter{
			hasMatchingSubscribers: func(context.Context, staterouter.HasMatchingSubscribersRequest) (bool, error) {
				return false, nil
			},
			routePublish: func(context.Context, staterouter.RoutePublishRequest) error {
				return nil
			},
		}),
	)
	c.ID = "qos2-dup-reason"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	handler := NewClientHandler(c)

	readPacketAsync := func() (<-chan *packets.ControlPacket, <-chan error) {
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
		return readCh, errCh
	}
	awaitPacket := func(readCh <-chan *packets.ControlPacket, errCh <-chan error) *packets.ControlPacket {
		t.Helper()
		select {
		case cp := <-readCh:
			return cp
		case err := <-errCh:
			t.Fatalf("read packet: %v", err)
		case <-time.After(3 * time.Second):
			t.Fatal("timeout waiting for packet")
		}
		return nil
	}

	readCh, errCh := readPacketAsync()
	if err := handler.handlePublish(context.Background(), &packets.Publish{
		Topic:      "qos2/no-subscriber",
		QoS:        2,
		PacketID:   91,
		Payload:    []byte("value"),
		Properties: &packets.PublishProperties{},
	}); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	firstRec, ok := awaitPacket(readCh, errCh).Content.(*packets.Pubrec)
	if !ok {
		t.Fatal("expected first PUBREC")
	}
	if firstRec.ReasonCode != packets.PubrecNoMatchingSubscribers {
		t.Fatalf("expected first No Matching Subscribers, got 0x%X", firstRec.ReasonCode)
	}

	c.component.plugin = &brokerplugin.Plugins{
		PacketPlugin: brokerplugin.PacketPlugin{
			OnReceivedPublish: []brokerplugin.OnReceivedPublish{
				func(context.Context, string, *packets.Publish) error {
					pluginCalled = true
					return errors.New("plugin must not see duplicate")
				},
			},
		},
	}

	readCh, errCh = readPacketAsync()
	if err := handler.handlePublish(context.Background(), &packets.Publish{
		Topic:      "qos2/no-subscriber",
		QoS:        2,
		PacketID:   91,
		Duplicate:  true,
		Payload:    []byte("value"),
		Properties: &packets.PublishProperties{},
	}); err != nil {
		t.Fatalf("duplicate publish: %v", err)
	}
	if pluginCalled {
		t.Fatal("duplicate QoS2 PUBLISH must bypass received-publish plugin")
	}
	dupRec, ok := awaitPacket(readCh, errCh).Content.(*packets.Pubrec)
	if !ok {
		t.Fatal("expected duplicate PUBREC")
	}
	if dupRec.ReasonCode != packets.PubrecNoMatchingSubscribers {
		t.Fatalf("expected duplicate reason No Matching Subscribers, got 0x%X", dupRec.ReasonCode)
	}
}

func TestHandlePublishQoS2PacketIDReuseReusesStoredPubrecBeforePlugin(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	tests := []struct {
		name    string
		publish *packets.Publish
	}{
		{
			name: "new publish without DUP",
			publish: &packets.Publish{
				Topic:      "qos2/original",
				QoS:        2,
				PacketID:   92,
				Payload:    []byte("value"),
				Properties: &packets.PublishProperties{},
			},
		},
		{
			name: "DUP publish with different payload",
			publish: &packets.Publish{
				Topic:      "qos2/original",
				QoS:        2,
				PacketID:   92,
				Duplicate:  true,
				Payload:    []byte("changed"),
				Properties: &packets.PublishProperties{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c1, c2 := net.Pipe()
			defer c1.Close()
			defer c2.Close()

			pluginCalled := false
			c := NewClient(c1)
			c.ID = "qos2-packet-id-in-use"
			c.ctx, c.cancel = context.WithCancelCause(context.Background())
			c.component.plugin = &brokerplugin.Plugins{
				PacketPlugin: brokerplugin.PacketPlugin{
					OnReceivedPublish: []brokerplugin.OnReceivedPublish{
						func(context.Context, string, *packets.Publish) error {
							pluginCalled = true
							return nil
						},
					},
				},
			}
			handler := NewClientHandler(c)
			if _, err := handler.storeQoS2Publish(context.Background(), &packets.Publish{
				Topic:      "qos2/original",
				QoS:        2,
				PacketID:   92,
				Payload:    []byte("value"),
				Properties: &packets.PublishProperties{},
			}, packets.PubrecSuccess); err != nil {
				t.Fatalf("store qos2 publish: %v", err)
			}

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

			if err := handler.handlePublish(context.Background(), tt.publish); err != nil {
				t.Fatalf("handle publish: %v", err)
			}
			if pluginCalled {
				t.Fatal("duplicate qos2 packet-id during await-pubrel must bypass received-publish plugin")
			}

			select {
			case cp := <-readCh:
				pubrec, ok := cp.Content.(*packets.Pubrec)
				if !ok {
					t.Fatalf("expected PUBREC, got %T", cp.Content)
				}
				if pubrec.PacketID != 92 || pubrec.ReasonCode != packets.PubrecSuccess {
					t.Fatalf("unexpected PUBREC: %+v", pubrec)
				}
			case err := <-errCh:
				t.Fatalf("read packet: %v", err)
			case <-time.After(3 * time.Second):
				t.Fatal("timeout waiting for PUBREC")
			}
		})
	}
}

func TestHandlePublishQoS2PacketIDReuseAfterNegativePubrecTreatsAsNewMessage(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	pluginCalled := false
	c := NewClient(c1)
	c.ID = "qos2-reuse-after-negative-pubrec"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.component.plugin = &brokerplugin.Plugins{
		PacketPlugin: brokerplugin.PacketPlugin{
			OnReceivedPublish: []brokerplugin.OnReceivedPublish{
				func(context.Context, string, *packets.Publish) error {
					pluginCalled = true
					return errors.New("reject")
				},
			},
		},
	}
	handler := NewClientHandler(c)

	// Simulate previous QoS2 stage-1 result as an error PUBREC.
	if _, err := handler.storeQoS2Publish(context.Background(), &packets.Publish{
		Topic:      "qos2/original",
		QoS:        2,
		PacketID:   93,
		Duplicate:  false,
		Payload:    []byte("value"),
		Properties: &packets.PublishProperties{},
	}, packets.PubrecNotAuthorized); err != nil {
		t.Fatalf("store qos2 publish: %v", err)
	}

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

	if err := handler.handlePublish(context.Background(), &packets.Publish{
		Topic:      "qos2/original",
		QoS:        2,
		PacketID:   93,
		Duplicate:  true,
		Payload:    []byte("value"),
		Properties: &packets.PublishProperties{},
	}); err != nil {
		t.Fatalf("handle publish: %v", err)
	}

	if !pluginCalled {
		t.Fatal("expected publish to be treated as a new message and passed to plugin")
	}

	select {
	case cp := <-readCh:
		pubrec, ok := cp.Content.(*packets.Pubrec)
		if !ok {
			t.Fatalf("expected PUBREC, got %T", cp.Content)
		}
		if pubrec.PacketID != 93 || pubrec.ReasonCode != packets.PubrecNotAuthorized {
			t.Fatalf("unexpected PUBREC: %+v", pubrec)
		}
	case err := <-errCh:
		t.Fatalf("read packet: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for PUBREC")
	}
}

func TestHandlePublishRetainFailureDoesNotCacheQoS1Ack(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	delivered := 0
	c := NewClient(c1, WithStateRouter(newTestPublishStateRouter(func(context.Context, *brokerpublish.Message) error {
		delivered++
		return nil
	})))
	c.ID = "qos1-retain-fail"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	handler := NewClientHandler(c)

	err := handler.handlePublish(context.Background(), &packets.Publish{
		Topic:      "retain/fail",
		QoS:        1,
		PacketID:   92,
		Retain:     true,
		Payload:    []byte("value"),
		Properties: &packets.PublishProperties{},
	})
	if err == nil {
		t.Fatal("expected retain store error")
	}
	if delivered != 0 {
		t.Fatalf("retain failure must stop live delivery before ACK caching, delivered=%d", delivered)
	}

	readCh := make(chan *packets.ControlPacket, 1)
	errCh := make(chan error, 1)
	go func() {
		_ = c2.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
		cp, err := wire.Decode(c2, wire.DecodeOptions{})
		if err != nil {
			errCh <- err
			return
		}
		readCh <- cp
	}()

	err = handler.handlePublish(context.Background(), &packets.Publish{
		Topic:      "retain/fail",
		QoS:        1,
		PacketID:   92,
		Duplicate:  true,
		Retain:     true,
		Payload:    []byte("value"),
		Properties: &packets.PublishProperties{},
	})
	if err == nil {
		t.Fatal("expected duplicate to retry and hit retain store error")
	}
	if delivered != 0 {
		t.Fatalf("duplicate must retry retained commit instead of using an uncommitted ACK, delivered=%d", delivered)
	}
	select {
	case cp := <-readCh:
		t.Fatalf("must not write cached PUBACK after retain failure, got %s", cp.PacketType())
	case <-errCh:
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for duplicate read result")
	}
}

func TestBuildUnsubAckReasonsConvertsNegativeAckToUnspecifiedError(t *testing.T) {
	unsuback := &packets.Unsuback{}
	ok, err := buildUnsubAckReasons(
		unsuback,
		[]string{"a/b"},
		[]byte{0},
		&proto.UnSubResponse{Topics: []int32{-1}},
	)
	if err != nil {
		t.Fatalf("negative unsubscribe ack should be mapped to a reason code, got error: %v", err)
	}
	if len(ok) != 0 {
		t.Fatalf("negative ack must not be marked successful: %+v", ok)
	}
	if len(unsuback.Reasons) != 1 || unsuback.Reasons[0] != packets.UnsubackUnspecifiedError {
		t.Fatalf("expected Unspecified Error reason, got %+v", unsuback.Reasons)
	}
}

func TestHandleUnsubReturnsNotAuthorizedWhenPluginRejects(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	called := false
	c := NewClient(c1, WithPlugin(&brokerplugin.Plugins{
		PacketPlugin: brokerplugin.PacketPlugin{
			OnUnsubscribe: []brokerplugin.OnUnsubscribe{
				func(_ context.Context, clientID string, unsubscribe *packets.Unsubscribe) error {
					called = true
					if clientID != "unsub-plugin-test" {
						t.Fatalf("expected client id unsub-plugin-test, got %q", clientID)
					}
					if len(unsubscribe.Topics) != 1 || unsubscribe.Topics[0] != "private/topic" {
						t.Fatalf("unexpected unsubscribe packet: %+v", unsubscribe)
					}
					return errors.New("deny unsubscribe")
				},
			},
		},
	}))
	c.ID = "unsub-plugin-test"
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

	err := NewClientHandler(c).handleUnsub(context.Background(), &packets.Unsubscribe{
		PacketID: 42,
		Topics:   []string{"private/topic"},
	})
	if err != nil {
		t.Fatalf("plugin rejection should be mapped to UNSUBACK, got error: %v", err)
	}
	if !called {
		t.Fatal("expected unsubscribe plugin to be called")
	}

	select {
	case cp := <-readCh:
		unsuback, ok := cp.Content.(*packets.Unsuback)
		if !ok {
			t.Fatalf("expected UNSUBACK, got %T", cp.Content)
		}
		if len(unsuback.Reasons) != 1 || unsuback.Reasons[0] != packets.UnsubackNotAuthorized {
			t.Fatalf("expected Not Authorized, got %+v", unsuback.Reasons)
		}
	case err := <-errCh:
		t.Fatalf("read packet error: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for UNSUBACK")
	}
}

// MQTT5 §3.1.2.11.7: even with RequestProblemInformation=0, the server
// MAY return ReasonString/UserProperty on CONNACK, DISCONNECT, and PUBLISH;
// it MUST NOT do so on PUBACK/PUBREC/PUBREL/PUBCOMP/SUBACK/UNSUBACK/AUTH.
func TestWriteSuppressesProblemInfoOnAckPacketsWhenClientRequestedNone(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	c := NewClient(c1)
	c.ID = "problem-info"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.requestProblemInfo = false

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

	pubackCP := packets.NewControlPacket(packets.PUBACK)
	pubackCP.Content = &packets.Puback{
		PacketID:   42,
		ReasonCode: packets.PubackNotAuthorized,
		Properties: &packets.PubackProperties{ReasonString: "do not expose"},
	}
	if err := c.write(&clientcap.WritePacket{Packet: pubackCP}); err != nil {
		t.Fatalf("write: %v", err)
	}

	select {
	case cp := <-readCh:
		puback, ok := cp.Content.(*packets.Puback)
		if !ok {
			t.Fatalf("expected PUBACK, got %T", cp.Content)
		}
		if puback.Properties != nil && puback.Properties.ReasonString != "" {
			t.Fatalf("expected reason string to be suppressed on PUBACK, got %q", puback.Properties.ReasonString)
		}
	case err := <-errCh:
		t.Fatalf("read packet error: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for PUBACK")
	}
}

// MQTT5 §3.1.2.11.7 lets the server keep ReasonString on DISCONNECT
// even when the client set RequestProblemInformation=0.
func TestWriteKeepsDisconnectReasonStringWhenClientRequestedNone(t *testing.T) {
	if err := func() error { _, err := config.Load("../../../etc/config.yaml"); return err }(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	c := NewClient(c1)
	c.ID = "problem-info"
	c.ctx, c.cancel = context.WithCancelCause(context.Background())
	c.requestProblemInfo = false

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

	if err := c.write(&clientcap.WritePacket{Packet: disconnectForProtocolError("keep this reason")}); err != nil {
		t.Fatalf("write: %v", err)
	}

	select {
	case cp := <-readCh:
		disc, ok := cp.Content.(*packets.Disconnect)
		if !ok {
			t.Fatalf("expected DISCONNECT, got %T", cp.Content)
		}
		if disc.Properties == nil || disc.Properties.ReasonString == "" {
			t.Fatalf("expected reason string preserved on DISCONNECT, got nil/empty")
		}
	case err := <-errCh:
		t.Fatalf("read packet error: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for DISCONNECT")
	}
}

func runPublishExpectDisconnect(t *testing.T, p *packets.Publish) *packets.ControlPacket {
	t.Helper()
	cp := runPublishExpectPacket(t, p)
	if _, ok := cp.Content.(*packets.Disconnect); !ok {
		t.Fatalf("expected DISCONNECT, got %T", cp.Content)
	}
	return cp
}

func runPublishExpectDisconnectWithBrokerConfig(t *testing.T, cfg config.Broker, p *packets.Publish) *packets.ControlPacket {
	t.Helper()
	cp := runPublishExpectPacketWithOptions(t, p, WithConfig(clientConfigForTest(cfg)))
	if _, ok := cp.Content.(*packets.Disconnect); !ok {
		t.Fatalf("expected DISCONNECT, got %T", cp.Content)
	}
	return cp
}

func runPublishExpectPacket(t *testing.T, p *packets.Publish) *packets.ControlPacket {
	t.Helper()
	return runPublishExpectPacketWithOptions(t, p)
}

func runPublishExpectPacketWithOptions(t *testing.T, p *packets.Publish, options ...ComponentOption) *packets.ControlPacket {
	t.Helper()
	logger.LoadForTest()

	c1, c2 := net.Pipe()
	defer c2.Close()

	c := NewClient(c1, options...)
	c.ID = "cap-test"
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

	handler := NewClientHandler(c)
	_ = handler.handlePublish(context.Background(), p)

	select {
	case cp := <-readCh:
		return cp
	case err := <-errCh:
		t.Fatalf("read packet error: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for DISCONNECT")
	}
	return nil
}
