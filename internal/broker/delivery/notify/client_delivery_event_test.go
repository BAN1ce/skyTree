package notify

import (
	"context"
	"testing"

	deliveryevent "github.com/BAN1ce/skyTree/internal/broker/delivery/event"
	"github.com/BAN1ce/skyTree/pkg/cluster"
	"github.com/BAN1ce/skyTree/pkg/eventbus"
)

type fakeNodeController struct {
	sharedWakeCalls int
	sharedWake      cluster.SharedSubscriptionWake
}

func (f *fakeNodeController) NotifyClientDelivery(context.Context, uint64, string, []string, int32, []byte, map[string]cluster.ClientDeliveryOptions) error {
	return nil
}

func (f *fakeNodeController) NotifySharedSubscriptionWake(_ context.Context, wake cluster.SharedSubscriptionWake) error {
	f.sharedWakeCalls++
	f.sharedWake = wake
	return nil
}

func (f *fakeNodeController) RequestCloseClient(context.Context, uint64, string, string) error {
	return nil
}

func TestAddListenerRejectsInvalidArguments(t *testing.T) {
	evt := New(1, nil, nil)
	if _, _, err := evt.AddListener(context.Background(), "c1", func(*deliveryevent.Notify) {}); err != ErrLocalEventCenterNil {
		t.Fatalf("expected ErrLocalEventCenterNil, got %v", err)
	}

	center := eventbus.NewEventCenter[*deliveryevent.Notify]()
	evt = New(1, center, nil)
	if _, _, err := evt.AddListener(context.Background(), "", func(*deliveryevent.Notify) {}); err != ErrClientIDEmpty {
		t.Fatalf("expected ErrClientIDEmpty, got %v", err)
	}
	if _, _, err := evt.AddListener(context.Background(), "c1", nil); err != ErrHandlerNil {
		t.Fatalf("expected ErrHandlerNil, got %v", err)
	}
}

func TestDeleteListenerRejectsInvalidArguments(t *testing.T) {
	evt := New(1, nil, nil)
	if err := evt.DeleteListener(context.Background(), "c1", "id1"); err != ErrLocalEventCenterNil {
		t.Fatalf("expected ErrLocalEventCenterNil, got %v", err)
	}

	center := eventbus.NewEventCenter[*deliveryevent.Notify]()
	evt = New(1, center, nil)
	if err := evt.DeleteListener(context.Background(), "", "id1"); err != ErrClientIDEmpty {
		t.Fatalf("expected ErrClientIDEmpty, got %v", err)
	}
	if err := evt.DeleteListener(context.Background(), "c1", ""); err != ErrListenerIDEmpty {
		t.Fatalf("expected ErrListenerIDEmpty, got %v", err)
	}
}

func TestNotifyToNodeRejectsEmptyClientIDs(t *testing.T) {
	center := eventbus.NewEventCenter[*deliveryevent.Notify]()
	evt := New(1, center, &fakeNodeController{})
	err := evt.NotifyToNode(context.Background(), 1, "topic/a", nil, deliveryevent.KindWake, nil, nil)
	if err != ErrClientIDsEmpty {
		t.Fatalf("expected ErrClientIDsEmpty, got %v", err)
	}
}

func TestNotifySharedWakeUsesTypedPayload(t *testing.T) {
	controller := &fakeNodeController{}
	evt := New(1, nil, controller)

	err := evt.NotifySharedWake(context.Background(), SharedWakePayload{
		ShareGroup:  "g1",
		TopicFilter: "a/b",
		TaskID:      "task-1",
	})
	if err != nil {
		t.Fatalf("NotifySharedWake: %v", err)
	}
	if controller.sharedWakeCalls != 1 {
		t.Fatalf("expected one shared wake call, got %d", controller.sharedWakeCalls)
	}
	if controller.sharedWake.ShareGroup != "g1" || controller.sharedWake.TopicFilter != "a/b" || controller.sharedWake.TaskID != "task-1" {
		t.Fatalf("unexpected shared wake: %+v", controller.sharedWake)
	}
}
