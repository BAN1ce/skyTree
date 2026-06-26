package client

import (
	"context"
	"fmt"
	"time"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/broker/staterouter"
	"github.com/BAN1ce/skyTree/logger"
	clientcap "github.com/BAN1ce/skyTree/pkg/brokerapi/clientcap"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription"
	"github.com/BAN1ce/skyTree/pkg/brokerapi/subscription"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/pkg/mqtt5/packetpool"
	topicutil "github.com/BAN1ce/skyTree/pkg/mqtt5/topic"
	proto2 "github.com/BAN1ce/skyTree/proto/proto_topic"
)

// handleSub 处理客户端 SUBSCRIBE 包。
//
// 它会先做 ACL 和能力校验，再写入 sub-center，最后按原订阅顺序返回 SUBACK，
// 并异步补发 retained message。
func (i *InnerHandler) handleSub(ctx context.Context, subscribe *packets.Subscribe) error {
	subAck, subAckContent := newSubAckPacket(subscribe.PacketID)
	if handled, err := i.rejectSubscribeByPlugin(ctx, subscribe, subAck, subAckContent); handled || err != nil {
		return err
	}

	precheck, err := i.precheckSubscribe(subscribe)
	if err != nil {
		return err
	}

	existedSessionSubs := i.readExistingSessionSubscriptions(subscribe)

	if precheck.allRejected() {
		appendRejectedSubAckReasons(subAckContent, precheck.rejectedReasons)
		_ = i.client.write(&clientcap.WritePacket{Packet: subAck})
		return nil
	}

	grantedQoS, err := i.createSubscriptions(precheck.subscribeForCreate(subscribe))
	if err != nil {
		return err
	}

	brokerCfg := i.client.brokerRuntimeConfig()
	ackResult := buildSubAckReasons(subAckContent, subscribe, precheck.rejectedReasons, grantedQoS, &brokerCfg.ConnectAckProperty)
	if err := i.client.write(&clientcap.WritePacket{Packet: subAck}); err != nil {
		return err
	}

	i.registerSuccessfulSharedSubscriptions(ackResult.successSubs, ackResult.successGrantedQoS)
	if len(ackResult.successSubs) > 0 {
		go i.sendRetainedAfterSubscribe(i.client.getCtx(), &packets.Subscribe{Subscriptions: ackResult.successSubs}, existedSessionSubs)
	}
	return nil
}

type subscribePrecheckResult struct {
	rejectedReasons []byte
	hasRejected     bool
	filtered        *packets.Subscribe
}

// newSubAckPacket 创建 SUBACK 包及其 payload。
func newSubAckPacket(packetID uint16) (*packets.ControlPacket, *packets.Suback) {
	subAck := packets.NewControlPacket(packets.SUBACK)
	subAckContent := &packets.Suback{PacketID: packetID}
	subAck.Content = subAckContent
	return subAck, subAckContent
}

// rejectSubscribeByPlugin 执行订阅插件检查并在拒绝时直接回复 SUBACK。
func (i *InnerHandler) rejectSubscribeByPlugin(
	ctx context.Context,
	subscribe *packets.Subscribe,
	subAck *packets.ControlPacket,
	subAckContent *packets.Suback,
) (bool, error) {
	if i.client.component == nil || i.client.component.plugin == nil {
		return false, nil
	}
	if err := i.client.component.plugin.DoReceivedSubscribe(ctx, i.client.getID(), subscribe); err != nil {
		appendRepeatedSubAckReason(subAckContent, len(subscribe.Subscriptions), packets.SubackNotauthorized)
		_ = i.client.write(&clientcap.WritePacket{Packet: subAck})
		return true, nil
	}
	return false, nil
}

// precheckSubscribe 在创建订阅前完成语法和能力预校验。
func (i *InnerHandler) precheckSubscribe(subscribe *packets.Subscribe) (subscribePrecheckResult, error) {
	result := subscribePrecheckResult{
		rejectedReasons: make([]byte, len(subscribe.Subscriptions)),
	}
	if err := i.applySubscribeTopicValidation(subscribe, &result); err != nil {
		return result, err
	}
	if err := i.validateSubscriptionIdentifier(subscribe); err != nil {
		return result, err
	}
	if err := i.applySubscribeCapabilities(subscribe, &result); err != nil {
		return result, err
	}
	result.filtered = buildFilteredSubscribe(subscribe, result.rejectedReasons, result.hasRejected)
	return result, nil
}

// applySubscribeTopicValidation 校验每个 Topic Filter 的语法和共享订阅约束。
func (i *InnerHandler) applySubscribeTopicValidation(subscribe *packets.Subscribe, result *subscribePrecheckResult) error {
	for idx, sub := range subscribe.Subscriptions {
		if invalidSubscribeTopicFilter(sub.Topic) {
			result.reject(idx, packets.SubackTopicFilterinvalid)
			continue
		}
		if sharedsubscription.IsSharedSubscription(sub.Topic) && sub.NoLocal {
			return i.failProtocol("No Local is not allowed on shared subscriptions")
		}
	}
	return nil
}

// validateSubscriptionIdentifier 校验 MQTT5 Subscription Identifier 取值范围。
func (i *InnerHandler) validateSubscriptionIdentifier(subscribe *packets.Subscribe) error {
	if subscribe.Properties == nil || subscribe.Properties.SubscriptionIdentifier == nil {
		return nil
	}
	sid := *subscribe.Properties.SubscriptionIdentifier
	if sid >= mqtt5SubscriptionIdentifierMin && sid <= mqtt5SubscriptionIdentifierMax {
		return nil
	}
	return i.failProtocolWithCode(
		packets.DisconnectMalformedPacket,
		fmt.Sprintf("subscription identifier %d out of range [%d, %d]", sid, mqtt5SubscriptionIdentifierMin, mqtt5SubscriptionIdentifierMax),
	)
}

// applySubscribeCapabilities 校验服务端是否支持通配符、共享订阅和订阅标识符。
func (i *InnerHandler) applySubscribeCapabilities(subscribe *packets.Subscribe, result *subscribePrecheckResult) error {
	brokerCfg := i.client.brokerRuntimeConfig()
	prop := &brokerCfg.ConnectAckProperty
	hasSubID := subscribe.Properties != nil && subscribe.Properties.SubscriptionIdentifier != nil
	if !prop.SubscriptionIdentifierAvailable && hasSubID {
		return i.failProtocolWithCode(
			packets.DisconnectSubscriptionIdentifiersNotSupported,
			"subscription identifiers are not supported",
		)
	}
	for idx, sub := range subscribe.Subscriptions {
		if result.rejectedReasons[idx] != 0 || sub.Topic == "" {
			continue
		}
		if topicutil.HasWildcard(sub.Topic) && !prop.WildcardSubscriptionAvailable {
			return i.failProtocolWithCode(
				packets.DisconnectWildcardSubscriptionsNotSupported,
				fmt.Sprintf("wildcard subscriptions are not supported: %s", sub.Topic),
			)
		}
		sharedUnavailable := sharedsubscription.IsSharedSubscription(sub.Topic) &&
			(!prop.SharedSubscriptionAvailable || !runtimeSharedSubscriptionAvailable(i.client.component))
		if sharedUnavailable {
			return i.failProtocolWithCode(
				packets.DisconnectSharedSubscriptionNotSupported,
				fmt.Sprintf("shared subscriptions are not supported: %s", sub.Topic),
			)
		}
	}
	return nil
}

// buildFilteredSubscribe 过滤掉预校验被拒绝的订阅项。
func buildFilteredSubscribe(
	subscribe *packets.Subscribe,
	rejectedReasons []byte,
	hasRejected bool,
) *packets.Subscribe {
	if !hasRejected {
		return nil
	}
	filteredSubs := make([]packets.SubOptions, 0, len(subscribe.Subscriptions))
	for idx, sub := range subscribe.Subscriptions {
		if rejectedReasons[idx] == 0 {
			filteredSubs = append(filteredSubs, sub)
		}
	}
	if len(filteredSubs) == 0 {
		return nil
	}
	return &packets.Subscribe{
		PacketID:      subscribe.PacketID,
		Properties:    subscribe.Properties,
		Subscriptions: filteredSubs,
	}
}

// reject 标记单个订阅项为拒绝状态并记录 reason code。
func (r *subscribePrecheckResult) reject(idx int, reason byte) {
	r.rejectedReasons[idx] = reason
	r.hasRejected = true
}

// allRejected 判断预校验后是否没有任何可创建的订阅项。
func (r subscribePrecheckResult) allRejected() bool {
	return r.hasRejected && r.filtered == nil
}

// subscribeForCreate 返回需要实际写入 sub-center 的订阅请求。
func (r subscribePrecheckResult) subscribeForCreate(subscribe *packets.Subscribe) *packets.Subscribe {
	if r.filtered != nil {
		return r.filtered
	}
	return subscribe
}

// readExistingSessionSubscriptions 读取当前会话已存在订阅，用于 Retain Handling 判定。
func (i *InnerHandler) readExistingSessionSubscriptions(subscribe *packets.Subscribe) map[string]bool {
	existedSessionSubs := make(map[string]bool)
	if len(subscribe.Subscriptions) == 0 || i.client.component == nil || i.client.component.stateRouter == nil {
		return existedSessionSubs
	}
	subsResp, err := i.client.component.stateRouter.ListClientSubscriptions(
		i.client.getCtx(),
		staterouter.ListClientSubscriptionsRequest{
			BrokerNodeID: i.client.clusterRuntimeConfig().LocalNodeID,
			ClientID:     i.client.getID(),
		},
	)
	if err != nil {
		logger.Logger.Warn().Err(err).Str("client", i.client.metaString()).Msg("failed to read subscriptions for RH decision")
		return existedSessionSubs
	}
	if subsResp == nil || subsResp.Topics == nil {
		return existedSessionSubs
	}
	currentSubs := subsResp.Topics
	for _, sub := range subscribe.Subscriptions {
		if sub.Topic != "" && currentSubs[sub.Topic] != nil {
			existedSessionSubs[sub.Topic] = true
		}
	}
	return existedSessionSubs
}

// createSubscriptions 逐条创建订阅并收集 sub-center 返回的授权 QoS。
func (i *InnerHandler) createSubscriptions(subscribe *packets.Subscribe) ([]int32, error) {
	if i.client.component == nil || i.client.component.stateRouter == nil {
		return nil, fmt.Errorf("state router is nil")
	}
	rsp, err := i.client.component.stateRouter.Subscribe(i.client.ctx, staterouter.SubscribeRequest{
		BrokerNodeID: i.client.clusterRuntimeConfig().LocalNodeID,
		ClientID:     i.client.getID(),
		OwnerToken:   i.client.getOwnerToken(),
		Subscribe:    subscribe,
		MaxQoS:       i.client.brokerRuntimeConfig().ConnectAckProperty.MaxQos,
	})
	if err != nil {
		logger.Logger.Error().Err(err).Str("client", i.client.getID()).Msg("create sub error")
		return nil, err
	}
	return rsp.GrantedQoS, nil
}

type subAckBuildResult struct {
	successSubs       []packets.SubOptions
	successGrantedQoS []int32
}

// buildSubAckReasons 生成与原始订阅顺序对齐的 SUBACK reason code 列表。
func buildSubAckReasons(
	subAckContent *packets.Suback,
	subscribe *packets.Subscribe,
	rejectedReasons []byte,
	grantedQoS []int32,
	prop *config.ConnectAckProperty,
) subAckBuildResult {
	successSubs := make([]packets.SubOptions, 0, len(subscribe.Subscriptions))
	successGranted := make([]int32, 0, len(subscribe.Subscriptions))
	validIdx := 0
	for idx, sub := range subscribe.Subscriptions {
		if rejectedReasons[idx] != 0 {
			subAckContent.Reasons = append(subAckContent.Reasons, rejectedReasons[idx])
			continue
		}
		if validIdx >= len(grantedQoS) {
			subAckContent.Reasons = append(subAckContent.Reasons, 0x80)
			continue
		}
		ack := subscribeAckQoS(sub, grantedQoS[validIdx], prop)
		validIdx++
		if ack < 0 {
			subAckContent.Reasons = append(subAckContent.Reasons, 0x80)
			continue
		}
		subAckContent.Reasons = append(subAckContent.Reasons, subscription.Int32ToQoS(ack))
		successSubs = append(successSubs, sub)
		successGranted = append(successGranted, ack)
	}
	return subAckBuildResult{successSubs: successSubs, successGrantedQoS: successGranted}
}

// subscribeAckQoS 对授权 QoS 进行服务端最大 QoS 限幅。
func subscribeAckQoS(sub packets.SubOptions, granted int32, prop *config.ConnectAckProperty) int32 {
	ack := granted
	if ack < 0 {
		return ack
	}
	if prop != nil && ack > int32(prop.MaxQos) {
		ack = int32(prop.MaxQos)
	}
	return ack
}

// registerSuccessfulSharedSubscriptions 将成功的共享订阅注册到共享订阅管理器。
func (i *InnerHandler) registerSuccessfulSharedSubscriptions(subs []packets.SubOptions, grantedQoS []int32) {
	if i.client.component == nil || i.client.component.sharedSubscriptionManager == nil || len(grantedQoS) == 0 {
		return
	}
	for idx, sub := range subs {
		if sub.Topic == "" || !sharedsubscription.IsSharedSubscription(sub.Topic) {
			continue
		}
		if idx >= len(grantedQoS) {
			continue
		}
		ack := grantedQoS[idx]
		if ack < 0 {
			continue
		}
		shareGroup, actualTopicFilter, err := sharedsubscription.ParseSharedSubscription(sub.Topic)
		if err != nil {
			logger.Logger.Warn().Err(err).Str("topic", sub.Topic).Str("client", i.client.getID()).Msg("failed to parse shared subscription")
			continue
		}
		if err := i.client.component.sharedSubscriptionManager.OnClientOnline(
			i.client.ctx,
			i.client.getID(),
			shareGroup,
			actualTopicFilter,
		); err != nil {
			logger.Logger.Warn().
				Err(err).
				Str("shareGroup", shareGroup).
				Str("client", i.client.getID()).
				Msg("failed to register shared subscription")
		}
	}
}

// appendRepeatedSubAckReason 追加 count 个相同的 SUBACK reason code。
func appendRepeatedSubAckReason(subAckContent *packets.Suback, count int, reason byte) {
	for range count {
		subAckContent.Reasons = append(subAckContent.Reasons, reason)
	}
}

// appendRejectedSubAckReasons 按顺序追加预校验阶段生成的拒绝原因码。
func appendRejectedSubAckReasons(subAckContent *packets.Suback, rejectedReasons []byte) {
	for _, reason := range rejectedReasons {
		subAckContent.Reasons = append(subAckContent.Reasons, reason)
	}
}

// invalidSubscribeTopicFilter 判断 SUBSCRIBE Topic Filter 是否违反 MQTT 通配符规则。
func invalidSubscribeTopicFilter(filter string) bool {
	return topicutil.ValidateTopicFilterSyntax(filter) != nil
}

// sendRetainedAfterSubscribe 在订阅成功后按 MQTT5 Retain Handling 规则补发 retained 消息。
//
// 这里替代旧的 topic-store 拉取链路，不再需要为每个 subTopic 启动独立 runner。
func (i *InnerHandler) sendRetainedAfterSubscribe(ctx context.Context, subscribe *packets.Subscribe, existedSessionSubs map[string]bool) {
	if !i.canSendRetainedAfterSubscribe(subscribe) {
		return
	}

	currentSessionSubs := cloneSessionSubscriptionState(existedSessionSubs)

	// 每个成功订阅单独判断 RH 规则；通配符订阅需要先扫描匹配到的 retained topic。
	for _, sub := range subscribe.Subscriptions {
		if ctx != nil && ctx.Err() != nil {
			return
		}
		if shouldSendRetainedForSubscription(sub, currentSessionSubs) {
			i.sendRetainedForTopicFilter(ctx, sub.Topic)
		}
		markSubscriptionAsExistingForRetainHandling(sub, &currentSessionSubs)
	}
}

// canSendRetainedAfterSubscribe 判断当前连接是否可以执行 retained 补发。
func (i *InnerHandler) canSendRetainedAfterSubscribe(subscribe *packets.Subscribe) bool {
	if i == nil || i.client == nil || i.client.component == nil || i.client.component.retain == nil {
		return false
	}
	return i.mqtt5RetainAvailable() && subscribe != nil && len(subscribe.Subscriptions) > 0
}

// shouldSendRetainedForSubscription 根据 Retain Handling 策略决定是否补发 retained。
func shouldSendRetainedForSubscription(sub packets.SubOptions, existedSessionSubs map[string]bool) bool {
	// MQTT5: shared subscriptions do not receive retained messages on subscribe.
	if sharedsubscription.IsSharedSubscription(sub.Topic) {
		return false
	}
	if sub.Topic == "" || sub.RetainHandling == 0x02 {
		return false
	}
	return sub.RetainHandling != 0x01 || existedSessionSubs == nil || !existedSessionSubs[sub.Topic]
}

// cloneSessionSubscriptionState 深拷贝订阅存在性状态表。
func cloneSessionSubscriptionState(in map[string]bool) map[string]bool {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]bool, len(in))
	for topicFilter, existed := range in {
		out[topicFilter] = existed
	}
	return out
}

// markSubscriptionAsExistingForRetainHandling 在 RH=1 时标记该订阅已存在。
func markSubscriptionAsExistingForRetainHandling(sub packets.SubOptions, existedSessionSubs *map[string]bool) {
	if sub.RetainHandling != 0x01 || sub.Topic == "" || sharedsubscription.IsSharedSubscription(sub.Topic) {
		return
	}
	if *existedSessionSubs == nil {
		*existedSessionSubs = make(map[string]bool)
	}
	(*existedSessionSubs)[sub.Topic] = true
}

// sendRetainedForTopicFilter 处理单个 topic filter 的 retained 补发入口。
func (i *InnerHandler) sendRetainedForTopicFilter(ctx context.Context, topicFilter string) {
	topicFilter = retainedTopicFilterForSubscription(topicFilter)
	if topicFilter == "" {
		return
	}
	if !topicutil.HasWildcard(topicFilter) {
		i.sendRetainedForTopic(ctx, topicFilter)
		return
	}
	i.sendRetainedForWildcardFilter(ctx, topicFilter)
}

// sendRetainedForWildcardFilter 扫描并补发通配符匹配到的 retained 消息。
func (i *InnerHandler) sendRetainedForWildcardFilter(ctx context.Context, topicFilter string) {
	retainedMessages, err := i.client.component.retain.GetRetainMessagesByTopicFilter(topicFilter)
	if err != nil {
		logger.Logger.Warn().Err(err).Str("topicFilter", topicFilter).Msg("failed to scan retained messages")
		return
	}
	for _, retained := range retainedMessages {
		if retained == nil || retained.GetTopic() == "" {
			continue
		}
		i.sendRetainedForTopic(ctx, retained.GetTopic())
	}
}

// retainedTopicFilterForSubscription 将共享订阅过滤器映射成真实 topic filter。
func retainedTopicFilterForSubscription(topicFilter string) string {
	if !sharedsubscription.IsSharedSubscription(topicFilter) {
		return topicFilter
	}
	_, actualTopicFilter, err := sharedsubscription.ParseSharedSubscription(topicFilter)
	if err != nil {
		return ""
	}
	return actualTopicFilter
}

// getRetainedMessageSubscriptionOptions 查询 retained 消息对应的订阅选项。
//
// 返回值包含最终投递 QoS、是否保留原 retain 标记，以及需要附加的 Subscription Identifier。
func (i *InnerHandler) getRetainedMessageSubscriptionOptions(ctx context.Context, topicName, publisherClientID string) (winnerQoS int32, winnerRAP bool, subIDs []int32, ok bool) {
	// 默认按原消息 retain 标记投递。
	winnerQoS = 0
	winnerRAP = true
	subIDs = nil
	matches, ok := i.retainedMatchesForClient(ctx, topicName, publisherClientID)
	if !ok {
		return winnerQoS, winnerRAP, subIDs, false
	}
	return chooseRetainedSubscriptionOptions(matches)
}

// retainedMatchesForClient 读取当前客户端对 retained topic 的匹配订阅集合。
func (i *InnerHandler) retainedMatchesForClient(ctx context.Context, topicName, publisherClientID string) ([]*proto2.MatchedSubscription, bool) {
	if i.client.component == nil || i.client.component.subCenter == nil {
		return nil, false
	}
	resp, err := i.client.component.subCenter.GetAllMatchClientV2(ctx, &proto2.GetAllMatchClientV2Request{Topic: topicName})
	if err != nil || resp == nil {
		return nil, false
	}
	clientID := i.client.getID()
	for _, cm := range resp.GetMatches() {
		if cm == nil || cm.GetClientID() != clientID {
			continue
		}
		matches := filterNoLocalRetainedMatches(cm.GetMatched(), clientID, publisherClientID)
		return matches, len(matches) > 0
	}
	return nil, false
}

type retainedSubscriptionWinner struct {
	qos         int32
	rap         bool
	filter      string
	wildcardCnt int
	depth       int
	set         bool
}

// chooseRetainedSubscriptionOptions 按优先级规则挑选 retained 投递使用的订阅参数。
func chooseRetainedSubscriptionOptions(matches []*proto2.MatchedSubscription) (int32, bool, []int32, bool) {
	winner := retainedSubscriptionWinner{rap: true}
	subIDs := make([]int32, 0)
	for _, ms := range matches {
		if ms == nil {
			continue
		}
		if winner.better(ms) {
			winner.update(ms)
		}
		if sid := ms.GetSubscriptionIdentifier(); sid > 0 {
			subIDs = append(subIDs, sid)
		}
	}
	return winner.qos, winner.rap, subIDs, winner.set
}

// better 比较两个候选订阅，决定 retained 投递参数优先级。
func (w retainedSubscriptionWinner) better(ms *proto2.MatchedSubscription) bool {
	qos := ms.GetQoS()
	filter := ms.GetTopicFilter()
	wildcardCnt, depth := retainedFilterMetrics(filter)
	if !w.set {
		return true
	}
	if qos != w.qos {
		return qos > w.qos
	}
	if wildcardCnt != w.wildcardCnt {
		return wildcardCnt < w.wildcardCnt
	}
	if depth != w.depth {
		return depth > w.depth
	}
	return filter < w.filter
}

// update 用候选订阅刷新 retained 投递的当前最优解。
func (w *retainedSubscriptionWinner) update(ms *proto2.MatchedSubscription) {
	w.qos = ms.GetQoS()
	w.rap = ms.GetRetainAsPublished()
	w.filter = ms.GetTopicFilter()
	w.wildcardCnt, w.depth = retainedFilterMetrics(w.filter)
	w.set = true
}

// retainedFilterMetrics 计算过滤器的通配符数量和层级深度。
func retainedFilterMetrics(filter string) (wildcardCnt int, depth int) {
	for _, ch := range filter {
		if ch == '+' || ch == '#' {
			wildcardCnt++
		}
	}
	if filter == "" {
		return wildcardCnt, 0
	}
	depth = 1
	for _, ch := range filter {
		if ch == '/' {
			depth++
		}
	}
	return wildcardCnt, depth
}

// filterNoLocalRetainedMatches 过滤掉 No Local 规则不允许的 retained 匹配结果。
func filterNoLocalRetainedMatches(matches []*proto2.MatchedSubscription, clientID, publisherClientID string) []*proto2.MatchedSubscription {
	if len(matches) == 0 {
		return nil
	}
	out := make([]*proto2.MatchedSubscription, 0, len(matches))
	for _, ms := range matches {
		if ms == nil {
			continue
		}
		if publisherClientID != "" && publisherClientID == clientID && ms.GetNoLocal() {
			continue
		}
		out = append(out, ms)
	}
	return out
}

// applySubscriptionOptionsToPublish 把 MQTT5 订阅选项应用到即将投递的 PUBLISH。
//
// 主要处理 RAP(Retain As Published) 和 Subscription Identifier。
func applySubscriptionOptionsToPublish(publishContent *packets.Publish, winnerRAP bool, subIDs []int32) {
	if publishContent == nil {
		return
	}

	// RAP=false 时，本次投递强制把 retain 标记清零。
	if !winnerRAP {
		publishContent.Retain = false
	}
	setPublishSubscriptionIdentifiers(publishContent, subIDs)
}

// sendRetainedForTopic 向当前客户端发送某个具体 topic 的 retained PUBLISH。
//
// 投递前会复制原始包、应用订阅选项，并按最终 QoS 分配 PacketID 与限流 token。
func (i *InnerHandler) sendRetainedForTopic(ctx context.Context, topicName string) {
	if i == nil || i.client == nil {
		return
	}
	if !i.mqtt5RetainAvailable() {
		return
	}
	if ctx != nil && ctx.Err() != nil {
		return
	}
	rm := i.getRetainMessage(topicName)
	if rm == nil || rm.GetControlPacket() == nil {
		return
	}

	publisherClientID := rm.SendClientID
	winnerQoS, winnerRAP, subIDs, ok := i.getRetainedMessageSubscriptionOptions(ctx, topicName, publisherClientID)
	if !ok {
		return
	}

	// 获取原始 ControlPacket，后续只在副本上改动。
	originalCP := rm.GetControlPacket()
	if originalCP == nil {
		return
	}

	// 使用对象池减少 retained 投递时的临时分配。
	newPublishPacket := packetpool.PublishPool.Get()
	packetpool.CopyPublish(newPublishPacket, originalCP)

	// 写出完成后归还对象池。
	defer packetpool.PublishPool.Put(newPublishPacket)

	if publishContent, ok := newPublishPacket.Content.(*packets.Publish); ok {
		applySubscriptionOptionsToPublish(publishContent, winnerRAP, subIDs)
		if winnerQoS < 0 {
			winnerQoS = 0
		}
		if int32(publishContent.QoS) > winnerQoS {
			publishContent.QoS = byte(winnerQoS)
		}
	}

	i.client.writeRetainedPublish(ctx, newPublishPacket, topicName)
}

// mqtt5RetainAvailable 判断当前 broker 是否启用 retained 能力。
func (i *InnerHandler) mqtt5RetainAvailable() bool {
	return i.client.brokerRuntimeConfig().ConnectAckProperty.RetainAvailable != 0
}

// handleUnsub 处理客户端 UNSUBSCRIBE 包。
//
// 它会过滤非法 Topic Filter，仅对合法项请求 sub-center 删除，并按入站顺序返回 UNSUBACK。
// 共享订阅成功取消后，还会同步通知共享订阅 manager。
func (i *InnerHandler) handleUnsub(ctx context.Context, unsubscribe *packets.Unsubscribe) error {
	unsubAck, unsubAckContent := newUnsubAckPacket(unsubscribe.PacketID)
	if handled, err := i.rejectUnsubscribeByPlugin(ctx, unsubscribe, unsubAck, unsubAckContent); handled || err != nil {
		return err
	}

	rejectedReasons, validTopics := precheckUnsubscribeTopics(unsubscribe.Topics)
	if len(validTopics) == 0 {
		appendUnsubAckReasons(unsubAckContent, rejectedReasons)
		return i.client.write(&clientcap.WritePacket{Packet: unsubAck})
	}

	rsp, err := i.deleteValidSubscriptions(validTopics)
	if err != nil {
		return err
	}
	successfullyUnsubscribed, err := buildUnsubAckReasons(unsubAckContent, unsubscribe.Topics, rejectedReasons, rsp)
	if err != nil {
		logger.Logger.Error().Err(err).Str("client", i.client.getID()).Msg("unsubscribe error")
		return err
	}
	i.unregisterSharedUnsubscriptions(ctx, validTopics, successfullyUnsubscribed)

	return i.client.write(&clientcap.WritePacket{
		Packet: unsubAck,
	})
}

// newUnsubAckPacket 创建 UNSUBACK 包及其 payload。
func newUnsubAckPacket(packetID uint16) (*packets.ControlPacket, *packets.Unsuback) {
	unsubAck := packets.NewControlPacket(packets.UNSUBACK)
	unsubAckContent := &packets.Unsuback{PacketID: packetID}
	unsubAck.Content = unsubAckContent
	return unsubAck, unsubAckContent
}

// rejectUnsubscribeByPlugin 执行取消订阅插件检查并在拒绝时回复 UNSUBACK。
func (i *InnerHandler) rejectUnsubscribeByPlugin(
	ctx context.Context,
	unsubscribe *packets.Unsubscribe,
	unsubAck *packets.ControlPacket,
	unsubAckContent *packets.Unsuback,
) (bool, error) {
	c := i.client
	if c.component == nil || c.component.plugin == nil {
		return false, nil
	}
	if err := c.component.plugin.DoReceivedUnsubscribe(ctx, c.getID(), unsubscribe); err != nil {
		appendRepeatedUnsubAckReason(unsubAckContent, len(unsubscribe.Topics), packets.UnsubackNotAuthorized)
		return true, i.client.write(&clientcap.WritePacket{Packet: unsubAck})
	}
	return false, nil
}

// precheckUnsubscribeTopics 过滤非法 topic filter 并保留合法项。
func precheckUnsubscribeTopics(topics []string) ([]byte, []string) {
	rejectedReasons := make([]byte, len(topics))
	validTopics := make([]string, 0, len(topics))
	for idx, topicFilter := range topics {
		if invalidUnsubscribeTopicFilter(topicFilter) {
			rejectedReasons[idx] = packets.UnsubackTopicFilterInvalid
			continue
		}
		validTopics = append(validTopics, topicFilter)
	}
	return rejectedReasons, validTopics
}

// deleteValidSubscriptions 删除通过语法校验后的合法订阅项。
func (i *InnerHandler) deleteValidSubscriptions(validTopics []string) (*proto2.UnSubResponse, error) {
	// 只把合法 Topic Filter 发送给 sub-center，避免一个非法项影响整个取消订阅流程。
	return i.client.component.subCenter.DeleteSub(i.client.ctx, &proto2.UnSubRequest{
		Topics:     validTopics,
		ClientID:   i.client.getID(),
		OwnerToken: i.client.getOwnerToken(),
	})
}

// buildUnsubAckReasons 构建与入站 topic 顺序一致的 UNSUBACK reason code。
func buildUnsubAckReasons(
	unsubAckContent *packets.Unsuback,
	topics []string,
	rejectedReasons []byte,
	rsp *proto2.UnSubResponse,
) (map[string]struct{}, error) {
	validReasonIdx := 0
	successfullyUnsubscribed := make(map[string]struct{}, len(topics))
	for idx, topicFilter := range topics {
		if rejectedReasons[idx] != 0 {
			unsubAckContent.Reasons = append(unsubAckContent.Reasons, rejectedReasons[idx])
			continue
		}
		if validReasonIdx >= len(rsp.Topics) {
			unsubAckContent.Reasons = append(unsubAckContent.Reasons, packets.UnsubackUnspecifiedError)
			continue
		}
		ack := rsp.Topics[validReasonIdx]
		validReasonIdx++
		if ack == -1 {
			unsubAckContent.Reasons = append(unsubAckContent.Reasons, packets.UnsubackUnspecifiedError)
			continue
		}
		unsubAckContent.Reasons = append(unsubAckContent.Reasons, byte(ack))
		if byte(ack) == packets.UnsubackSuccess {
			successfullyUnsubscribed[topicFilter] = struct{}{}
		}
	}
	return successfullyUnsubscribed, nil
}

// appendUnsubAckReasons 追加预处理阶段的 UNSUBACK reason code。
func appendUnsubAckReasons(unsubAckContent *packets.Unsuback, reasons []byte) {
	for _, reason := range reasons {
		unsubAckContent.Reasons = append(unsubAckContent.Reasons, reason)
	}
}

// appendRepeatedUnsubAckReason 追加 count 个相同的 UNSUBACK reason code。
func appendRepeatedUnsubAckReason(unsubAckContent *packets.Unsuback, count int, reason byte) {
	for range count {
		unsubAckContent.Reasons = append(unsubAckContent.Reasons, reason)
	}
}

// unregisterSharedUnsubscriptions 从共享订阅管理器中注销成功取消的共享订阅。
func (i *InnerHandler) unregisterSharedUnsubscriptions(ctx context.Context, validTopics []string, successfullyUnsubscribed map[string]struct{}) {
	// 取消共享订阅时，同步移除共享组中的在线消费者。
	if i.client.component == nil || i.client.component.sharedSubscriptionManager == nil {
		return
	}
	for _, topicFilter := range validTopics {
		if _, ok := successfullyUnsubscribed[topicFilter]; !ok || topicFilter == "" {
			continue
		}
		if !sharedsubscription.IsSharedSubscription(topicFilter) {
			continue
		}
		shareGroup, topicName, err := sharedsubscription.ParseSharedSubscription(topicFilter)
		if err != nil {
			logger.Logger.Warn().Err(err).Str("topic", topicFilter).Str("client", i.client.getID()).Msg("failed to parse shared subscription")
			continue
		}
		_ = i.client.component.sharedSubscriptionManager.OnClientUnsubscribe(ctx, i.client.getID(), shareGroup, topicName)
	}
}

// invalidUnsubscribeTopicFilter 判断 UNSUBSCRIBE Topic Filter 是否非法。
//
// 普通订阅沿用 SUBSCRIBE 的 Topic Filter 规则；共享订阅还要校验解析出的真实过滤器。
func invalidUnsubscribeTopicFilter(filter string) bool {
	return invalidSubscribeTopicFilter(filter)
}

// mqtt5ReasonCodeIsError 判断 MQTT5 ReasonCode 是否表示错误。
func mqtt5ReasonCodeIsError(reasonCode byte) bool {
	return reasonCode >= 0x80
}

// getRetainMessage 读取 retained 消息并转换成可直接投递的 PUBLISH。
//
// 如果 retained 消息已经过期，会顺手清理存储并返回 nil。
func (i *InnerHandler) getRetainMessage(topicName string) *brokerpublish.Message {
	if message, ok := i.client.component.retain.GetRetainMessage(topicName); ok {
		publish, available := publishFromRetainMessage(message, time.Now())
		if !available {
			if message.GetTopic() != "" {
				_ = i.client.component.retain.DeleteRetainMessage(message.GetTopic())
			}
			return nil
		}

		logger.Logger.Debug().Str("topic", topicName).Msg("retain message publish")
		pub := packets.NewControlPacket(packets.PUBLISH)
		pub.Content = publish

		msg := &brokerpublish.Message{
			SendClientID: message.GetPublisherClientID(),
			Retain:       true,
		}
		msg.SetControlPacket(pub)
		return msg
	}
	return nil
}

// restoreSharedSubscriptions 在客户端重连后恢复它的共享订阅在线关系。
func (i *InnerHandler) restoreSharedSubscriptions(ctx context.Context) {
	if i.client.component == nil || i.client.component.subCenter == nil || i.client.component.sharedSubscriptionManager == nil {
		return
	}

	// 从 sub-center 读取当前客户端的全部订阅，再筛出共享订阅。
	resp, err := i.client.component.subCenter.GetClientSubscriptions(ctx, &proto2.GetClientSubscriptionsRequest{
		ClientID: i.client.getID(),
	})
	if err != nil {
		logger.Logger.Warn().Err(err).Str("client", i.client.getID()).Msg("failed to get client subscriptions for restore")
		return
	}

	if resp == nil || resp.Topics == nil {
		return
	}

	// 将共享订阅重新注册到本节点 manager，恢复共享组内的在线消费者视图。
	for topicFilter, subOption := range resp.Topics {
		if subOption == nil || topicFilter == "" {
			continue
		}

		// 只恢复共享订阅，普通订阅已由 sub-center 持久化。
		if sharedsubscription.IsSharedSubscription(topicFilter) {
			shareGroup, actualTopicFilter, err := sharedsubscription.ParseSharedSubscription(topicFilter)
			if err != nil {
				logger.Logger.Warn().Err(err).Str("topic", topicFilter).Str("client", i.client.getID()).Msg("failed to parse shared subscription during restore")
				continue
			}

			// 把客户端重新注册到共享组。
			if err := i.client.component.sharedSubscriptionManager.OnClientOnline(ctx, i.client.getID(), shareGroup, actualTopicFilter); err != nil {
				logger.Logger.Warn().Err(err).Str("shareGroup", shareGroup).Str("client", i.client.getID()).Msg("failed to restore shared subscription")
			} else {
				logger.Logger.Debug().Str("shareGroup", shareGroup).Str("topicFilter", actualTopicFilter).Str("client", i.client.getID()).Msg("restored shared subscription")
			}
		}
	}
}
