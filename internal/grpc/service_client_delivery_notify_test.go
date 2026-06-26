package grpc

import (
	"context"
	"testing"

	delivery_event "github.com/BAN1ce/skyTree/internal/broker/delivery/event"
	delivery_notify "github.com/BAN1ce/skyTree/internal/broker/delivery/notify"
	shared_manager "github.com/BAN1ce/skyTree/internal/broker/sharedsubscription/manager"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/grpc/nodepb"
	"github.com/google/uuid"
)

type recordingSharedWakeHandler struct {
	calls int
	event shared_manager.TaskAppendedEvent
}

func (h *recordingSharedWakeHandler) HandleRemoteTaskAppended(_ context.Context, event shared_manager.TaskAppendedEvent) error {
	h.calls++
	h.event = event
	return nil
}

func TestClientDeliveryNotifyHandlesSharedWakeWithoutClientIDs(t *testing.T) {
	taskID := uuid.New()
	payload, err := delivery_notify.EncodeSharedWakePayload(delivery_notify.SharedWakePayload{
		ShareGroup:  "g1",
		TopicFilter: "a/b",
		TaskID:      taskID.String(),
	})
	if err != nil {
		t.Fatalf("encode shared wake: %v", err)
	}
	handler := &recordingSharedWakeHandler{}
	svc := NewClientDeliveryNotifyGRPCServer(nil, func() sharedWakeHandler { return handler })

	if _, err := svc.Notify(context.Background(), &nodepb.ClientDeliveryNotifyRequest{
		Kind:    nodepb.ClientDeliveryNotifyKind(delivery_event.KindSharedWake),
		Payload: payload,
	}); err != nil {
		t.Fatalf("Notify shared wake: %v", err)
	}
	if handler.calls != 1 {
		t.Fatalf("expected one shared wake call, got %d", handler.calls)
	}
	if handler.event.ShareGroup != "g1" || handler.event.TopicFilter != "a/b" || handler.event.TaskID != taskID {
		t.Fatalf("unexpected shared wake event: %+v", handler.event)
	}
}
