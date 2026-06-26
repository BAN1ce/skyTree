package grpc

import (
	"context"
	"fmt"

	"github.com/BAN1ce/skyTree/internal/broker/delivery/event"
	delivery_notify "github.com/BAN1ce/skyTree/internal/broker/delivery/notify"
	shared_manager "github.com/BAN1ce/skyTree/internal/broker/sharedsubscription/manager"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/grpc/nodepb"
	"github.com/BAN1ce/skyTree/pkg/eventbus"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/emptypb"
)

// ServiceClientDeliveryNotify handles node-to-node notifications for client delivery runner.
type ServiceClientDeliveryNotify struct {
	nodepb.UnimplementedClientDeliveryNotifyServer
	localEvent *eventbus.EventCenter[*delivery_event.Notify]
	sharedWake sharedWakeProvider
}

type sharedWakeHandler interface {
	HandleRemoteTaskAppended(ctx context.Context, event shared_manager.TaskAppendedEvent) error
}

type sharedWakeProvider func() sharedWakeHandler

func NewClientDeliveryNotifyGRPCServer(center *eventbus.EventCenter[*delivery_event.Notify], sharedWake sharedWakeProvider) *ServiceClientDeliveryNotify {
	return &ServiceClientDeliveryNotify{localEvent: center, sharedWake: sharedWake}
}

func (s *ServiceClientDeliveryNotify) Notify(ctx context.Context, request *nodepb.ClientDeliveryNotifyRequest) (*emptypb.Empty, error) {
	if request == nil {
		return &emptypb.Empty{}, nil
	}

	kind := delivery_event.Kind(request.GetKind())
	if kind == delivery_event.KindSharedWake {
		return s.handleSharedWake(ctx, request)
	}

	if s.localEvent == nil || len(request.GetClientIDs()) == 0 {
		return &emptypb.Empty{}, nil
	}
	baseNotify, emitKey, err := delivery_notify.BuildPreparedNotify(kind, request.GetPublishTopic(), request.GetPayload())
	if err != nil {
		return nil, err
	}
	options := toClientDeliveryOptions(request.GetClientOptions())

	emittedCount := 0
	noListenerCount := 0
	skippedCount := 0
	failedCount := 0
	for _, clientID := range request.GetClientIDs() {
		if clientID == "" {
			skippedCount++
			continue
		}
		eventName := eventbus.ReceiveClientDeliveryEventName(clientID)
		if s.localEvent.EventListenerCount(eventName) == 0 {
			noListenerCount++
		}
		notifyPayload := delivery_notify.BuildClientNotifyPayload(baseNotify, clientID, options)
		if err := s.localEvent.Emit(eventName, emitKey, notifyPayload); err != nil {
			failedCount++
			continue
		}
		emittedCount++
	}
	logger.Logger.Debug().
		Int("kind", int(kind)).
		Int("client_count", len(request.GetClientIDs())).
		Int("emitted_count", emittedCount).
		Int("no_listener_count", noListenerCount).
		Int("skipped_count", skippedCount).
		Int("failed_count", failedCount).
		Msg("client delivery notify emit result")

	if emittedCount == 0 || noListenerCount == emittedCount {
		return nil, fmt.Errorf("client delivery notify emitted no listeners: no_listener=%d failed=%d skipped=%d", noListenerCount, failedCount, skippedCount)
	}
	return &emptypb.Empty{}, nil
}

func (s *ServiceClientDeliveryNotify) handleSharedWake(ctx context.Context, request *nodepb.ClientDeliveryNotifyRequest) (*emptypb.Empty, error) {
	if s.sharedWake == nil {
		return &emptypb.Empty{}, nil
	}
	handler := s.sharedWake()
	if handler == nil {
		return &emptypb.Empty{}, nil
	}
	payload, err := delivery_notify.DecodeSharedWakePayload(request.GetPayload())
	if err != nil {
		return nil, err
	}
	taskID, err := uuid.Parse(payload.TaskID)
	if err != nil {
		return nil, err
	}
	if err := handler.HandleRemoteTaskAppended(ctx, shared_manager.TaskAppendedEvent{
		ShareGroup:  payload.ShareGroup,
		TopicFilter: payload.TopicFilter,
		TaskID:      taskID,
	}); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func toClientDeliveryOptions(in map[string]*nodepb.ClientDeliveryOptions) map[string]delivery_notify.ClientDeliveryOptions {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]delivery_notify.ClientDeliveryOptions, len(in))
	for clientID, opts := range in {
		if clientID == "" || opts == nil {
			continue
		}
		out[clientID] = delivery_notify.ClientDeliveryOptions{
			NoLocal:             opts.GetNoLocal(),
			RAP:                 opts.GetRAP(),
			SubscriptionIDsJSON: opts.GetSubscriptionIDsJSON(),
		}
	}
	return out
}
