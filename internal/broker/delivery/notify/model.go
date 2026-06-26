package notify

import (
	"encoding/json"
	"fmt"
	"time"

	deliveryevent "github.com/BAN1ce/skyTree/internal/broker/delivery/event"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/message/serializer"
	"github.com/BAN1ce/skyTree/proto/proto_event"
	"github.com/google/uuid"
	proto2 "google.golang.org/protobuf/proto"
)

type PreparedNotify struct {
	EmitKey      string
	Kind         deliveryevent.Kind
	PublishTopic string
	Message      interface{}
}

type SharedWakePayload struct {
	ShareGroup  string `json:"shareGroup"`
	TopicFilter string `json:"topicFilter"`
	TaskID      string `json:"taskID"`
}

func EncodeSharedWakePayload(payload SharedWakePayload) ([]byte, error) {
	if payload.ShareGroup == "" {
		return nil, fmt.Errorf("shared wake shareGroup is empty")
	}
	if payload.TopicFilter == "" {
		return nil, fmt.Errorf("shared wake topicFilter is empty")
	}
	if payload.TaskID == "" {
		return nil, fmt.Errorf("shared wake taskID is empty")
	}
	return json.Marshal(payload)
}

func DecodeSharedWakePayload(data []byte) (SharedWakePayload, error) {
	if len(data) == 0 {
		return SharedWakePayload{}, fmt.Errorf("shared wake payload is empty")
	}
	var payload SharedWakePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return SharedWakePayload{}, err
	}
	if payload.ShareGroup == "" {
		return SharedWakePayload{}, fmt.Errorf("shared wake shareGroup is empty")
	}
	if payload.TopicFilter == "" {
		return SharedWakePayload{}, fmt.Errorf("shared wake topicFilter is empty")
	}
	if payload.TaskID == "" {
		return SharedWakePayload{}, fmt.Errorf("shared wake taskID is empty")
	}
	return payload, nil
}

func BuildPreparedNotify(kind deliveryevent.Kind, publishTopic string, payload []byte) (*deliveryevent.Notify, string, error) {
	emitKey := buildEmitKey(kind, publishTopic)
	switch kind {
	case deliveryevent.KindWake:
		return &deliveryevent.Notify{
			Kind:         kind,
			PublishTopic: publishTopic,
		}, emitKey, nil
	case deliveryevent.KindQoS0Direct:
		if len(payload) == 0 {
			return nil, "", fmt.Errorf("qos0 direct notify requires payload")
		}
		req := &proto_event.Request{}
		if err := proto2.Unmarshal(payload, req); err != nil {
			return nil, "", err
		}
		if req.GetID() != "" {
			emitKey = req.GetID()
		}
		msg, err := serializer.Serializer.Decode(req.GetData())
		if err != nil {
			return nil, "", err
		}
		return &deliveryevent.Notify{
			Kind:         kind,
			PublishTopic: publishTopic,
			Message:      msg,
		}, emitKey, nil
	default:
		return &deliveryevent.Notify{
			Kind:         kind,
			PublishTopic: publishTopic,
		}, emitKey, nil
	}
}

func BuildClientNotifyPayload(base *deliveryevent.Notify, clientID string, clientOptions map[string]ClientDeliveryOptions) *deliveryevent.Notify {
	if base == nil {
		return nil
	}
	if base.Kind != deliveryevent.KindQoS0Direct {
		return &deliveryevent.Notify{
			Kind:         base.Kind,
			PublishTopic: base.PublishTopic,
		}
	}
	out := &deliveryevent.Notify{
		Kind:         base.Kind,
		PublishTopic: base.PublishTopic,
		Message:      base.Message,
		NoLocal:      false,
		RAP:          true,
	}
	if clientOptions != nil {
		if opts, ok := clientOptions[clientID]; ok {
			out.NoLocal = opts.NoLocal
			out.RAP = opts.RAP
			out.SubscriptionIDsJSON = opts.SubscriptionIDsJSON
		}
	}
	return out
}

func buildEmitKey(kind deliveryevent.Kind, publishTopic string) string {
	if kind == deliveryevent.KindWake {
		return fmt.Sprintf("%s-%d", publishTopic, time.Now().UnixNano())
	}
	return uuid.NewString()
}
