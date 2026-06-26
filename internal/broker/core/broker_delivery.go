package core

import (
	"context"
	"errors"
	"time"

	brokerclient "github.com/BAN1ce/skyTree/internal/broker/client"
	"github.com/BAN1ce/skyTree/internal/broker/delivery"
	"github.com/BAN1ce/skyTree/logger"
	brokerpublish "github.com/BAN1ce/skyTree/pkg/brokerapi/publish"
	packets "github.com/BAN1ce/skyTree/pkg/mqtt5"
	"github.com/BAN1ce/skyTree/proto/proto_session"
)

// RoutePublish 将 client 侧 publish 事件接入 Broker 的下行投递流水线。
func (b *Broker) RoutePublish(ctx context.Context, m *brokerpublish.Message) error {
	return b.routePublishClientMode(ctx, m)
}

// clearPublishedWillFromSession 在遗嘱消息成功投递后清理 session 中保存的 will 状态。
func (b *Broker) clearPublishedWillFromSession(ctx context.Context, willMsg *brokerpublish.Message) {
	if b == nil || b.state.sessionCenter == nil || willMsg == nil || willMsg.SendClientID == "" {
		return
	}
	sessionResp, err := b.state.sessionCenter.GetSession(ctx, &proto_session.ReadSessionRequest{
		ClientID: willMsg.SendClientID,
	})
	if err != nil || sessionResp == nil || !sessionResp.GetExist() || sessionResp.GetSession() == nil {
		return
	}
	sess := sessionResp.GetSession()
	if sess.GetWillMessage() == nil {
		return
	}
	if err := b.state.sessionCenter.SaveOfflineState(ctx, &proto_session.SaveOfflineStateRequest{
		ClientID:              willMsg.SendClientID,
		UnfinishedMessages:    sess.GetUnfinishedMessages(),
		ClearWill:             true,
		SessionExpiryInterval: sess.GetSessionExpiryInterval(),
		NowUnixNano:           time.Now().UnixNano(),
		OwnerToken:            willMsg.OwnerToken,
	}); err != nil {
		logger.Logger.Warn().
			Err(err).
			Str("client", willMsg.SendClientID).
			Msg("failed to clear will message from session after publish")
	}
}

// routePublishClientMode 负责将一次 publish 路由为普通 client 任务或共享订阅任务。
func (b *Broker) routePublishClientMode(ctx context.Context, m *brokerpublish.Message) error {
	publish := m.Publish
	if publish == nil {
		return nil
	}
	if b.delivery.taskStore == nil {
		return errors.New("delivery task store is nil")
	}
	if err := b.processRetainedInternalPublish(publish, m.SendClientID); err != nil {
		return err
	}

	routeRes, err := b.routePublishDelivery(ctx, publish, m.SendClientID)
	if err != nil {
		return err
	}
	if !hasDeliveryRoutes(routeRes) {
		return nil
	}

	if publish.QoS == 0 && !b.config.broker.StoreQoS0 {
		return b.handleQoS0DirectNoStore(ctx, m, publish, routeRes)
	}

	now := time.Now()
	messageID, err := b.delivery.taskStore.SavePublishMessage(ctx, now, m.SendClientID, publish, m.MessageID)
	if err != nil {
		return err
	}
	if err := b.appendSharedDeliveryTasks(ctx, now, publish.Topic, messageID, routeRes.ShareGroupTasks); err != nil {
		return err
	}
	if err := b.appendClientDeliveryTasks(ctx, now, publish.Topic, messageID, routeRes.Plans); err != nil {
		return err
	}
	b.wakeClientDeliveryRunners(ctx, publish.Topic, routeRes.Plans)
	return nil
}

func (b *Broker) processRetainedInternalPublish(publish *packets.Publish, publisherClientID string) error {
	if publish == nil || !publish.Retain {
		return nil
	}
	if b == nil || b.state.retain == nil {
		return errors.New("retain store is nil")
	}
	if len(publish.Payload) == 0 {
		return b.state.retain.DeleteRetainMessage(publish.Topic)
	}
	retained, err := brokerclient.NewRetainMessageFromPublish(publish, time.Now(), publisherClientID)
	if err != nil {
		return err
	}
	return b.state.retain.PutRetainMessage(retained)
}

// routePublishDelivery 基于订阅中心计算当前 publish 的普通订阅和共享订阅目标。
func (b *Broker) routePublishDelivery(ctx context.Context, publish *packets.Publish, senderClientID string) (*delivery.RouteResult, error) {
	router := delivery.NewSubCenterRouter(b.state.subCenter)
	return router.Route(ctx, publish, senderClientID)
}

func hasDeliveryRoutes(routeRes *delivery.RouteResult) bool {
	return routeRes != nil && (len(routeRes.Plans) > 0 || len(routeRes.ShareGroupTasks) > 0)
}

// clientOptions 汇总 Broker 持有的依赖，构造每个新连接对应的 client 组件配置。
func (b *Broker) clientOptions() []brokerclient.ComponentOption {
	opts := []brokerclient.ComponentOption{
		brokerclient.WithConfig(
			brokerclient.Config{
				WindowSize:           10,
				ReadStoreTimeout:     3 * time.Second,
				WriteTimeout:         5 * time.Second,
				KeepAlive:            time.Duration(b.config.broker.KeepAlive) * time.Second,
				BrokerConfig:         b.config.broker,
				BrokerConfigResolved: true,
				ClusterConfig:        b.config.cluster,
				DeliveryConfig:       b.config.delivery,
			}),
		brokerclient.WithPlugin(b.pluginSet.hooks),
		brokerclient.WithRetain(b.state.retain),
		brokerclient.WithSubCenter(b.state.subCenter),
		brokerclient.WithLifecycleContext(b.runtime.ctx),
		brokerclient.WithBackgroundTaskWaitGroup(&b.clients.backgroundTaskWg),
		brokerclient.WithDeliveryCursorStore(b.delivery.cursorStore),
		brokerclient.WithClientDeliveryEvent(b.delivery.event),
		brokerclient.WithSessionCenter(b.state.sessionCenter),
		brokerclient.WithKeepAliveTracker(b.clients.keepAliveTracker),
		brokerclient.WithStateRouter(b.state.router),
		brokerclient.WithNotifyWillMessageChan(b.will.messageChan),
		brokerclient.WithClosClient(b.integrations.nodeController),
		brokerclient.WithClientManager(b.clients.manager),
	}
	if b.publish.retry != nil {
		opts = append(opts, brokerclient.WithPublishRetry(b.publish.retry))
	}
	if b.will.delayCenter != nil {
		opts = append(opts, brokerclient.WithWillDelayCenter(b.will.delayCenter))
	}
	if b.shared.manager != nil {
		opts = append(opts, brokerclient.WithSharedSubscriptionManager(b.shared.manager))
	}
	return opts
}
