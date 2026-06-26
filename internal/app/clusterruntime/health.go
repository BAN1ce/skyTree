package clusterruntime

import (
	"context"

	"github.com/BAN1ce/skyTree/config"
	inner_cluster "github.com/BAN1ce/skyTree/internal/clusterhealth"
	"github.com/BAN1ce/skyTree/logger"
	raft2 "github.com/BAN1ce/skyTree/pkg/cluster/raft"
	"github.com/BAN1ce/skyTree/pkg/cluster/raftcfg"
	"github.com/kataras/go-events"
)

// healthEventEmitterAdapter 将本地事件总线适配为健康检查事件发射器。
type healthEventEmitterAdapter struct {
	driver events.EventEmmiter
}

// Emit 将健康检查事件转发到本地事件总线。
func (a healthEventEmitterAdapter) Emit(eventName string, payload interface{}) {
	if a.driver == nil {
		return
	}
	a.driver.Emit(events.EventName(eventName), payload)
}

// BuildHealthChecker 根据集群状态和配置创建健康检查器。
func BuildHealthChecker(
	ctx context.Context,
	cfg config.AppConfig,
	cluster *raft2.Cluster,
	eventDriver events.EventEmmiter,
) *inner_cluster.HealthChecker {
	hcCfg, ok := NewHealthCheckConfig(cfg, cluster)
	if !ok {
		return nil
	}

	var emitter inner_cluster.HealthEventEmitter
	if eventDriver != nil {
		emitter = healthEventEmitterAdapter{driver: eventDriver}
	}
	return inner_cluster.NewHealthChecker(ctx, cluster, hcCfg, emitter)
}

// NewHealthCheckConfig 解析健康检查配置，并过滤不可启动健康检查的场景。
func NewHealthCheckConfig(
	cfg config.AppConfig,
	cluster *raft2.Cluster,
) (inner_cluster.HealthCheckConfig, bool) {
	if !cfg.Cluster.Enable || cluster == nil {
		return inner_cluster.HealthCheckConfig{}, false
	}

	configHC := raftcfg.HealthCheckWithDefaults(cfg.Cluster.HealthCheck)
	if !configHC.Enabled {
		return inner_cluster.HealthCheckConfig{}, false
	}

	hostedClusterIDs := cluster.StartedClusterIDs()
	if len(hostedClusterIDs) == 0 {
		return inner_cluster.HealthCheckConfig{}, false
	}

	return inner_cluster.HealthCheckConfig{
		Enabled:          configHC.Enabled,
		Interval:         configHC.Interval,
		Timeout:          configHC.Timeout,
		MaxRetries:       configHC.MaxRetries,
		HostedClusterIDs: hostedClusterIDs,
	}, true
}

// RegisterHealthCheckEventListeners 注册健康检查事件日志监听器。
func RegisterHealthCheckEventListeners(localEvent events.EventEmmiter) {
	if localEvent == nil {
		return
	}

	localEvent.AddListener(inner_cluster.HealthCheckSuccessEvent, func(data ...interface{}) {
		event, ok := decodeEventPayload[inner_cluster.HealthStatusEvent](data)
		if !ok {
			return
		}
		logger.Logger.Info().
			Uint64("cluster_id", event.ClusterID).
			Str("cluster_name", event.ClusterName).
			Str("current_status", inner_cluster.HealthStatusText(event.CurrentStatus)).
			Msg("cluster health check succeeded")
	})

	localEvent.AddListener(inner_cluster.HealthCheckFailureEvent, func(data ...interface{}) {
		event, ok := decodeEventPayload[inner_cluster.HealthStatusEvent](data)
		if !ok {
			return
		}
		logger.Logger.Error().
			Uint64("cluster_id", event.ClusterID).
			Str("cluster_name", event.ClusterName).
			Str("current_status", inner_cluster.HealthStatusText(event.CurrentStatus)).
			Str("error", event.Error).
			Msg("cluster health check failed")
	})

	localEvent.AddListener(inner_cluster.HealthCheckRecoveryEvent, func(data ...interface{}) {
		event, ok := decodeEventPayload[inner_cluster.HealthRecoveryEvent](data)
		if !ok {
			return
		}
		logger.Logger.Info().
			Uint64("cluster_id", event.ClusterID).
			Str("cluster_name", event.ClusterName).
			Time("recovery_time", event.RecoveryTime).
			Dur("downtime", event.Downtime).
			Msg("cluster recovered from failure")
	})
}

// decodeEventPayload 从事件参数中解析指定类型的 payload。
func decodeEventPayload[T any](data []interface{}) (T, bool) {
	var zero T
	if len(data) == 0 {
		return zero, false
	}
	if event, ok := data[0].(T); ok {
		return event, true
	}
	if eventPtr, ok := data[0].(*T); ok && eventPtr != nil {
		return *eventPtr, true
	}
	return zero, false
}
