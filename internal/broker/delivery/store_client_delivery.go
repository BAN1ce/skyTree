package delivery

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/BAN1ce/skyTree/pkg/brokerapi/message/serializer"
	store "github.com/BAN1ce/skyTree/pkg/brokerapi/persistence"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/google/uuid"
)

type ClientDeliveryTaskStore struct {
	persistence *DeliveryPersistence
}

func NewClientDeliveryTaskStore(clientDeliveryStore store.ClientDeliveryStore) (*ClientDeliveryTaskStore, error) {
	persistence, err := NewDeliveryPersistence(clientDeliveryStore, serializer.Serializer)
	if err != nil {
		return nil, err
	}
	return &ClientDeliveryTaskStore{persistence: persistence}, nil
}

func (s *ClientDeliveryTaskStore) EnsureSchema(ctx context.Context) error {
	return s.persistence.EnsureSchema(ctx)
}

func (s *ClientDeliveryTaskStore) SavePublishMessage(ctx context.Context, ts time.Time, publisherClientID string, publish *packets.Publish, messageID uuid.UUID) (uuid.UUID, error) {
	if publish == nil {
		return uuid.Nil, fmt.Errorf("publish is nil")
	}
	createdNano := ts.UnixNano()
	// MQTT5 §3.3.2.3.3：在入库时把"剩余秒数"换算成绝对的过期时刻，
	// 之后无论重启、跨节点、重传都只看 ExpiredTime，避免重复减算被延长。
	var expiredNano int64
	if publish.Properties != nil && publish.Properties.MessageExpiry != nil {
		expiry := *publish.Properties.MessageExpiry
		if expiry == 0 {
			// MQTT5: Message Expiry Interval=0 表示消息已经过期。
			expiredNano = createdNano
		} else {
			expiredNano = createdNano + int64(expiry)*int64(time.Second)
		}
	}
	msg := &brokerpublish.Message{
		CreatedTime:  createdNano,
		ExpiredTime:  expiredNano,
		SendClientID: publisherClientID,
	}
	if messageID != uuid.Nil {
		msg.MessageID = messageID
	}
	msg.SetPublish(clonePublishForStorage(publish))
	return s.persistence.SaveMessagePayload(ctx, ts, publish.Topic, publisherClientID, msg)
}

func clonePublishForStorage(publish *packets.Publish) *packets.Publish {
	if publish == nil {
		return nil
	}
	cloned := *publish
	// PacketID is scoped to one client connection. Stored broker messages get a
	// fresh PacketID when each receiver's delivery runner sends them.
	cloned.PacketID = 0
	cloned.Payload = append([]byte(nil), publish.Payload...)
	if publish.Properties != nil {
		props := *publish.Properties
		props.PayloadFormat = clonePtr(publish.Properties.PayloadFormat)
		props.MessageExpiry = clonePtr(publish.Properties.MessageExpiry)
		props.WillDelayInterval = clonePtr(publish.Properties.WillDelayInterval)
		props.TopicAlias = clonePtr(publish.Properties.TopicAlias)
		props.CorrelationData = append([]byte(nil), publish.Properties.CorrelationData...)
		props.SubscriptionIdentifier = append([]int(nil), publish.Properties.SubscriptionIdentifier...)
		props.User = append([]packets.User(nil), publish.Properties.User...)
		cloned.Properties = &props
	}
	return &cloned
}

func clonePtr[T any](value *T) *T {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func (s *ClientDeliveryTaskStore) AppendClientTask(ctx context.Context, ts time.Time, clientID string, messageID uuid.UUID, plan ClientPlan) (uuid.UUID, bool, error) {
	task := store.DeliveryTask{
		TS:                ts,
		ClientID:          clientID,
		MessageID:         messageID,
		DeliveryQoS:       plan.DeliveryQoS,
		SubscriptionIDs:   subscriptionIDsFromJSON(plan.SubscriptionIDsJSON),
		NoLocal:           plan.WinnerNoLocal,
		RetainAsPublished: plan.WinnerRAP,
		ShareGroup:        plan.ShareGroup,
		SharedTaskID:      plan.SharedTaskID,
	}
	return s.persistence.AppendDeliveryTask(ctx, task)
}

func (s *ClientDeliveryTaskStore) ClientTaskExists(ctx context.Context, clientID string, messageID uuid.UUID) (bool, error) {
	return s.persistence.DeliveryTaskExists(ctx, clientID, messageID)
}

func subscriptionIDsFromJSON(raw string) []int32 {
	if raw == "" {
		return nil
	}
	var ids []int32
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil
	}
	return ids
}
