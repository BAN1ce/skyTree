package core

import (
	"context"
	"time"

	brokerclient "github.com/BAN1ce/skyTree/internal/broker/client"
	"github.com/BAN1ce/skyTree/logger"
)

const brokerRemoveNodeTimeout = 3 * time.Second

// Close 执行 Broker 的优雅关闭流程，先通知在线客户端，再停止后台组件并等待 client 退出。
func (b *Broker) Close() error {
	if b == nil {
		return nil
	}
	b.runtime.shuttingDown.Store(true)

	// MQTT5 §3.14.2.2.2：服务端关停时应当下发 DISCONNECT(0x8B) 通知所有在线客户端。
	b.broadcastServerShuttingDown()

	var err error
	if b.network.server != nil {
		err = b.network.server.Close()
	}
	if closeErr := b.closePublishRetry(); closeErr != nil {
		logger.Logger.Warn().Err(closeErr).Msg("failed to close publish retry worker")
	}
	if b.will.delayScanner != nil {
		b.will.delayScanner.Stop()
	}
	b.drainWillMessages(brokerWillDrainTimeout)

	if b.runtime.cancel != nil {
		b.runtime.cancel()
	}
	b.removeNodeFromClusterState()
	if b.shared.manager != nil {
		b.shared.manager.Stop()
	}
	if b.state.retain != nil {
		b.state.retain.StopGC()
	}

	b.clients.wg.Wait()
	b.clients.backgroundTaskWg.Wait()
	return err
}

// removeNodeFromClusterState 在 Broker 关闭时从集群状态中移除本节点，失败只记录日志避免阻塞退出。
func (b *Broker) removeNodeFromClusterState() {
	if b == nil || b.cluster.nodeState == nil || b.cluster.nodeMeta == nil || b.cluster.nodeMeta.LocalNodeID == 0 {
		return
	}
	nodeID := b.cluster.nodeMeta.LocalNodeID
	ctx, cancel := context.WithTimeout(context.Background(), brokerRemoveNodeTimeout)
	defer cancel()

	if err := b.cluster.nodeState.RemoveNode(ctx, nodeID); err != nil {
		if logger.Logger != nil {
			logger.Logger.Warn().Err(err).Uint64("node_id", nodeID).Msg("failed to remove node from cluster state")
		}
	}
}

// broadcastServerShuttingDown 向所有在线连接下发 DISCONNECT(0x8B Server Shutting Down)。
// 优雅关停已经显式发送 DISCONNECT，因此必须抑制遗嘱发布；出错只记日志，不阻塞关停流程。
func (b *Broker) broadcastServerShuttingDown() {
	if b == nil || b.clients.manager == nil {
		return
	}
	for _, c := range b.clients.manager.Snapshot() {
		if c == nil {
			continue
		}
		disc := brokerclient.DisconnectForServerShuttingDown()
		if err := c.CloseWithDisconnectNoWill(disc); err != nil {
			logger.Logger.Warn().Err(err).Str("client", c.GetID()).Msg("failed to send server-shutdown disconnect")
		}
		b.clients.manager.DeleteClient(c)
	}
}
