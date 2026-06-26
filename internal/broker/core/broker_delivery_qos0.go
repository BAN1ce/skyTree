package core

import (
	"context"
	"math/rand/v2"

	"github.com/BAN1ce/skyTree/internal/broker/delivery"
	deliveryevent "github.com/BAN1ce/skyTree/internal/broker/delivery/event"
	delivery_notify "github.com/BAN1ce/skyTree/internal/broker/delivery/notify"
	"github.com/BAN1ce/skyTree/internal/broker/sharedsubscription/domain"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/message/serializer"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/proto/proto_event"
	"github.com/google/uuid"
	proto2 "google.golang.org/protobuf/proto"
)

// handleQoS0DirectNoStore 在未开启 QoS0 存储时直接通知在线 owner，避免落库后再由 runner 拉取。
func (b *Broker) handleQoS0DirectNoStore(ctx context.Context, m *brokerpublish.Message, publish *packets.Publish, routeRes *delivery.RouteResult) error {
	if publish == nil || routeRes == nil {
		return nil
	}
	if b.delivery.event == nil || b.state.sessionCenter == nil {
		return nil
	}

	payload, err := b.encodeQoS0DirectPayload(m)
	if err != nil {
		return err
	}

	ownerCache := newOwnerNodeLookupCache()
	b.notifyQoS0DirectNormalPlans(ctx, publish, routeRes.Plans, payload, ownerCache)
	b.notifyQoS0DirectSharedTasks(ctx, publish, routeRes.ShareGroupTasks, payload, ownerCache)
	return nil
}

// encodeQoS0DirectPayload 将 QoS0 publish 封装为跨节点通知 payload，供远端 client 直接写出。
func (b *Broker) encodeQoS0DirectPayload(m *brokerpublish.Message) ([]byte, error) {
	encoded, err := serializer.Serializer.Encode(m)
	if err != nil {
		return nil, err
	}
	req := &proto_event.Request{
		ID:            uuid.NewString(),
		Type:          proto_event.RequestType_EMIT_EVENT,
		Data:          encoded,
		RequestNodeID: 0,
	}
	if b != nil && b.cluster.nodeMeta != nil {
		req.RequestNodeID = uint64(b.cluster.nodeMeta.LocalNodeID)
	}
	return proto2.Marshal(req)
}

// notifyQoS0DirectNormalPlans 按节点聚合普通订阅目标，并携带每个 client 的订阅选项。
func (b *Broker) notifyQoS0DirectNormalPlans(
	ctx context.Context,
	publish *packets.Publish,
	plans []delivery.ClientPlan,
	payload []byte,
	ownerCache *ownerNodeLookupCache,
) {
	if publish == nil || len(plans) == 0 || b == nil || b.delivery.event == nil || b.state.sessionCenter == nil {
		return
	}

	nodeByClient := b.resolveOnlineOwnerNodes(ctx, collectPlanClientIDs(plans), ownerCache)
	nodeClientSet := make(map[uint64]map[string]struct{}, 16)
	clientOptionsMap := make(map[string]delivery_notify.ClientDeliveryOptions, len(plans))
	for _, plan := range plans {
		if plan.ClientID == "" {
			continue
		}
		nodeID, ok := nodeByClient[plan.ClientID]
		if !ok {
			continue
		}
		set, ok := nodeClientSet[nodeID]
		if !ok {
			set = make(map[string]struct{}, 8)
			nodeClientSet[nodeID] = set
		}
		set[plan.ClientID] = struct{}{}
		clientOptionsMap[plan.ClientID] = delivery_notify.ClientDeliveryOptions{
			NoLocal:             plan.WinnerNoLocal,
			RAP:                 plan.WinnerRAP,
			SubscriptionIDsJSON: plan.SubscriptionIDsJSON,
		}
	}

	for nodeID, set := range nodeClientSet {
		clientIDs := make([]string, 0, len(set))
		nodeClientOptions := make(map[string]delivery_notify.ClientDeliveryOptions, len(set))
		for cid := range set {
			clientIDs = append(clientIDs, cid)
			if opts, ok := clientOptionsMap[cid]; ok {
				nodeClientOptions[cid] = opts
			}
		}
		_ = b.delivery.event.NotifyToNode(ctx, nodeID, publish.Topic, clientIDs, deliveryevent.KindQoS0Direct, payload, nodeClientOptions)
	}
}

// notifyQoS0DirectSharedTasks 为每个共享订阅任务挑选一个在线成员进行 QoS0 直投。
func (b *Broker) notifyQoS0DirectSharedTasks(
	ctx context.Context,
	publish *packets.Publish,
	shareTasks []delivery.ShareGroupTask,
	payload []byte,
	ownerCache *ownerNodeLookupCache,
) {
	if !b.canNotifyQoS0DirectSharedTasks(publish, shareTasks) {
		return
	}
	for _, shareTask := range shareTasks {
		selected, selectedOpts, ok := b.selectQoS0SharedCandidate(ctx, shareTask, ownerCache)
		if ok {
			b.notifyQoS0SharedCandidate(ctx, publish, payload, selected, selectedOpts)
		}
	}
}

type qos0SharedCandidate struct {
	clientID string
	nodeID   uint64
}

func (b *Broker) canNotifyQoS0DirectSharedTasks(publish *packets.Publish, shareTasks []delivery.ShareGroupTask) bool {
	return publish != nil &&
		len(shareTasks) > 0 &&
		b != nil &&
		b.delivery.event != nil &&
		b.state.sessionCenter != nil &&
		b.state.subCenter != nil
}

// selectQoS0SharedCandidate 从共享组在线成员中随机选择一个满足订阅选项的投递目标。
func (b *Broker) selectQoS0SharedCandidate(
	ctx context.Context,
	shareTask delivery.ShareGroupTask,
	ownerCache *ownerNodeLookupCache,
) (qos0SharedCandidate, delivery_notify.ClientDeliveryOptions, bool) {
	if shareTask.ShareGroup == "" || shareTask.TopicFilter == "" {
		return qos0SharedCandidate{}, delivery_notify.ClientDeliveryOptions{}, false
	}
	candidates := b.onlineQoS0SharedCandidates(ctx, shareTask, ownerCache)
	if len(candidates) == 0 {
		return qos0SharedCandidate{}, delivery_notify.ClientDeliveryOptions{}, false
	}
	for _, idx := range rand.Perm(len(candidates)) {
		selected := candidates[idx]
		selectedOpts, ok := b.resolveSharedDeliveryOptionsForClient(ctx, selected.clientID, shareTask)
		if ok {
			return selected, selectedOpts, true
		}
	}
	return qos0SharedCandidate{}, delivery_notify.ClientDeliveryOptions{}, false
}

// onlineQoS0SharedCandidates 查询共享组候选成员，并过滤出当前仍在线且有 owner 节点的 client。
func (b *Broker) onlineQoS0SharedCandidates(
	ctx context.Context,
	shareTask delivery.ShareGroupTask,
	ownerCache *ownerNodeLookupCache,
) []qos0SharedCandidate {
	strategy := domain.NewSharedDeliveryStrategy(b.state.subCenter, nil, nil)
	members, err := strategy.OnlineCandidates(ctx, domain.AssignTaskCommand{
		ShareGroup:      shareTask.ShareGroup,
		TopicFilter:     shareTask.TopicFilter,
		PublisherClient: shareTask.PublisherClient,
		PublishQoS:      shareTask.PublishQoS,
	})
	if err != nil || len(members) == 0 {
		return nil
	}

	candidateClientIDs := make([]string, 0, len(members))
	for _, member := range members {
		if member == nil || member.ClientID == "" {
			continue
		}
		candidateClientIDs = append(candidateClientIDs, member.ClientID)
	}
	nodeByClient := b.resolveOnlineOwnerNodes(ctx, candidateClientIDs, ownerCache)

	online := make([]qos0SharedCandidate, 0, 8)
	for _, member := range members {
		if member == nil || member.ClientID == "" {
			continue
		}
		if nodeID, ok := nodeByClient[member.ClientID]; ok {
			online = append(online, qos0SharedCandidate{clientID: member.ClientID, nodeID: nodeID})
		}
	}
	return online
}

func (b *Broker) notifyQoS0SharedCandidate(
	ctx context.Context,
	publish *packets.Publish,
	payload []byte,
	selected qos0SharedCandidate,
	selectedOpts delivery_notify.ClientDeliveryOptions,
) {
	opts := map[string]delivery_notify.ClientDeliveryOptions{
		selected.clientID: selectedOpts,
	}
	_ = b.delivery.event.NotifyToNode(ctx, selected.nodeID, publish.Topic, []string{selected.clientID}, deliveryevent.KindQoS0Direct, payload, opts)
}

// resolveSharedDeliveryOptionsForClient 解析共享订阅成员的 no-local、RAP 和订阅标识符配置。
func (b *Broker) resolveSharedDeliveryOptionsForClient(
	ctx context.Context,
	clientID string,
	shareTask delivery.ShareGroupTask,
) (delivery_notify.ClientDeliveryOptions, bool) {
	if b == nil || b.state.subCenter == nil || clientID == "" || shareTask.ShareGroup == "" || shareTask.TopicFilter == "" {
		return delivery_notify.ClientDeliveryOptions{}, false
	}
	strategy := domain.NewSharedDeliveryStrategy(b.state.subCenter, nil, nil)
	resolved, ok, err := strategy.ResolveClientOptions(ctx, domain.ResolveClientOptionsCommand{
		ShareGroup:      shareTask.ShareGroup,
		TopicFilter:     shareTask.TopicFilter,
		PublisherClient: shareTask.PublisherClient,
		PublishQoS:      shareTask.PublishQoS,
		ClientID:        clientID,
	})
	if err != nil || !ok {
		return delivery_notify.ClientDeliveryOptions{}, false
	}
	return delivery_notify.ClientDeliveryOptions{
		NoLocal:             resolved.NoLocal,
		RAP:                 resolved.RAP,
		SubscriptionIDsJSON: resolved.SubscriptionIDsJSON,
	}, true
}
