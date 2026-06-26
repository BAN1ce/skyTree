package core

import (
	"context"
	"net"
	"time"

	brokerclient "github.com/BAN1ce/skyTree/internal/broker/client"
	shared_manager "github.com/BAN1ce/skyTree/internal/broker/sharedsubscription/manager"
	will_delay2 "github.com/BAN1ce/skyTree/internal/broker/willdelay"
	"github.com/BAN1ce/skyTree/logger"
	shared_selector "github.com/BAN1ce/skyTree/pkg/brokerapi/sharedsubscription/selector"
	"github.com/BAN1ce/skyTree/pkg/cluster/raft"
)

func (b *Broker) Name() string {
	return "broker"
}

// Start 启动 MQTT Broker，并按顺序初始化集群注册、后台清理、共享订阅和遗嘱延迟扫描。
func (b *Broker) Start(ctx context.Context) error {
	b.runtime.ctx, b.runtime.cancel = context.WithCancel(ctx)
	b.runtime.shuttingDown.Store(false)
	var (
		err error
	)

	if err = b.startPublishRetry(b.runtime.ctx); err != nil {
		return err
	}
	if err = b.network.server.Start(b.runtime.ctx); err != nil {
		_ = b.closePublishRetry()
		return err
	}
	addNodeCtx, cancel := context.WithTimeout(b.runtime.ctx, 30*time.Second)
	for i := 0; i < 10; i++ {
		if err = b.cluster.nodeState.AddNode(addNodeCtx, b.cluster.nodeMeta); err == nil {
			logger.Logger.Info().Msg("add node to cluster state success")
			break
		} else {
			logger.Logger.Warn().Err(err).Msg("add node to cluster state failed")
		}
		time.Sleep(1 * time.Second)
	}
	cancel()

	if err != nil {
		return err
	}

	go b.deleteUnAliveClientTimeJob()

	go b.listenWillMessage()

	if _, ok := b.state.sessionCenter.(expiredSessionDeleter); ok {
		go b.deleteExpiredSessionsJob()
	}

	// MQTT5 §3.3.2.3.3：定期清理已过期的 retained 消息。
	if b.state.retain != nil {
		b.state.retain.StartGCWithPredicate(
			b.runtime.ctx,
			b.config.broker.Retain.GCInterval,
			func(context.Context) bool {
				return b.isRaftGroupLeader(raft.ClusterIDKeyStore)
			},
		)
	}

	// 共享订阅需要 consumer coordinator 消费队列；只有注入共享存储时才启动。
	// 否则 $share/... 订阅只会被保存，不会产生共享投递。
	if b.shared.store != nil && b.shared.manager == nil {
		nodeID := uint64(0)
		if b.cluster.nodeMeta != nil {
			nodeID = uint64(b.cluster.nodeMeta.LocalNodeID)
		}
		b.shared.manager = shared_manager.NewSharedSubscriptionManager(
			b.shared.store,
			b.state.sessionCenter,
			sharedSubClientManagerAdapter{m: b.clients.manager},
			shared_selector.NewRoundRobinSelector(),
			b.delivery.taskStore,
			b.delivery.cursorStore,
			b.state.subCenter,
			nodeID,
			b.cluster.nodeMeta,
			nil, // leaderElection 可选；任务状态通过 CAS 更新保证并发安全。
			b.delivery.event,
		)
	}
	if b.shared.manager != nil {
		_ = b.shared.manager.Start(b.runtime.ctx)
	}

	if b.will.delayCenter != nil && b.will.delaySessionCenter != nil {
		// scanner 依赖 Broker 生命周期 ctx，因此在 Start 阶段创建。
		b.will.delayScanner = will_delay2.NewScanner(
			b.runtime.ctx,
			b.will.delayCenter,
			b.will.delaySessionCenter,
			b.will.messageChan,
			b.cluster.raft,
			b.will.delayClusterID,
			b.will.delayLocalNodeID,
		)
		if b.will.delayScanner != nil {
			b.will.delayScanner.Start()
			logger.Logger.Info().Msg("started will delay scanner")
		}
	}

	b.acceptConn()
	return nil
}

func (b *Broker) startPublishRetry(ctx context.Context) error {
	if b == nil || b.publish.retryWorker == nil {
		return nil
	}
	return b.publish.retryWorker.StartSchedule(ctx)
}

func (b *Broker) closePublishRetry() error {
	if b == nil || b.publish.retryWorker == nil {
		return nil
	}
	return b.publish.retryWorker.Close()
}

// acceptConn 持续接收底层 server 的连接，为每个连接创建 client 并托管其生命周期。
func (b *Broker) acceptConn() {
	for {
		select {
		case <-b.runtime.ctx.Done():
			logger.Logger.Info().Msg("broker closing")

			// 复制连接快照后再逐个关闭，避免持锁执行网络关闭操作。
			b.network.mux.RLock()
			conn := make([]net.Conn, 0, len(b.network.connections))
			for c := range b.network.connections {
				conn = append(conn, c)
			}
			b.network.mux.RUnlock()

			for _, conn := range conn {
				if conn != nil {
					_ = conn.Close()
				} else {
					logger.Logger.Info().Msg("conn is nil")
				}
			}
			logger.Logger.Info().Msg("broker closed, close all client connection")

			return

		case conn, ok := <-b.network.server.Conn():
			if !ok {
				logger.Logger.Info().Msg("server closed")
				return
			}
			if conn == nil {
				logger.Logger.Error().Msg("conn is nil")
				continue
			}
			if b.runtime.shuttingDown.Load() {
				_ = conn.Close()
				continue
			}

			newClient := brokerclient.NewClient(conn, b.clientOptions()...)

			b.clients.wg.Add(1)

			go func(c *brokerclient.Client, acceptedConn net.Conn) {
				defer func() {
					b.network.mux.Lock()
					delete(b.network.connections, acceptedConn)
					b.clients.manager.DeleteClient(c)
					b.network.mux.Unlock()
					b.clients.wg.Done()
					logger.Logger.Info().Str("client", c.MetaString()).Msg("client closed")
				}()

				b.network.mux.Lock()
				b.network.connections[acceptedConn] = struct{}{}
				b.network.mux.Unlock()

				c.Run(b.runtime.ctx, []brokerclient.Handler{brokerclient.NewClientHandler(c)})

			}(newClient, conn)
		}
	}
}
