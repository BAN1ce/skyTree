package clusterhealth

import (
	"context"
	"sync"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	"github.com/BAN1ce/skyTree/pkg/cluster/healthcheck"
	"github.com/BAN1ce/skyTree/pkg/cluster/raft"
	"github.com/BAN1ce/skyTree/pkg/metric"
)

// HealthCheckEvent 健康检查事件类型
const (
	HealthCheckSuccessEvent  = "cluster.health.success"
	HealthCheckFailureEvent  = "cluster.health.failure"
	HealthCheckRecoveryEvent = "cluster.health.recovery"
)

// HealthEventEmitter emits health check lifecycle events.
type HealthEventEmitter interface {
	Emit(eventName string, payload interface{})
}

// HealthStatus 健康状态
type HealthStatus int

const (
	HealthStatusUnknown HealthStatus = iota
	HealthStatusHealthy
	HealthStatusUnhealthy
)

// HealthStatusText returns user-friendly text for health status.
func HealthStatusText(s HealthStatus) string {
	switch s {
	case HealthStatusHealthy:
		return "healthy"
	case HealthStatusUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// ClusterHealthInfo 集群健康信息
type ClusterHealthInfo struct {
	ClusterID    uint64        `json:"cluster_id"`
	ClusterName  string        `json:"cluster_name"`
	Status       HealthStatus  `json:"status"`
	LastCheck    time.Time     `json:"last_check"`
	LastSuccess  time.Time     `json:"last_success"`
	FailureCount int           `json:"failure_count"`
	Latency      time.Duration `json:"latency"`
	Error        string        `json:"error,omitempty"`
}

// HealthStatusEvent is emitted when a cluster transitions to healthy/unhealthy.
type HealthStatusEvent struct {
	ClusterID      uint64       `json:"cluster_id"`
	ClusterName    string       `json:"cluster_name"`
	PreviousStatus HealthStatus `json:"previous_status"`
	CurrentStatus  HealthStatus `json:"current_status"`
	Timestamp      time.Time    `json:"timestamp"`
	Error          string       `json:"error,omitempty"`
}

// HealthRecoveryEvent is emitted when an unhealthy cluster recovers.
type HealthRecoveryEvent struct {
	ClusterID    uint64        `json:"cluster_id"`
	ClusterName  string        `json:"cluster_name"`
	RecoveryTime time.Time     `json:"recovery_time"`
	Downtime     time.Duration `json:"downtime"`
}

// HealthChecker 集群健康检查器
type HealthChecker struct {
	ctx              context.Context
	cancel           context.CancelFunc
	cluster          *raft.Cluster
	clients          map[uint64]*raft.Client
	statuses         map[uint64]*ClusterHealthInfo
	systemClusterIDs []uint64
	mutex            sync.RWMutex
	interval         time.Duration
	timeout          time.Duration
	maxRetries       int
	enabled          bool
	emitter          HealthEventEmitter
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	// Enabled 是否启用健康检查协程；为 false 时 Start() 直接返回，不执行任何探测。
	Enabled bool `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	// Interval 健康检查周期；每隔该时长触发一轮 checkAllClusters（启动后会先立即执行一次）。
	Interval time.Duration `json:"interval" yaml:"interval" mapstructure:"interval"`
	// Timeout 单次集群探测超时；用于限制一次 Read(healthcheck) 的最长等待时间。
	Timeout time.Duration `json:"timeout" yaml:"timeout" mapstructure:"timeout"`
	// MaxRetries 连续失败阈值；同一集群失败次数达到该值后状态才会标记为 Unhealthy。
	MaxRetries int `json:"max_retries" yaml:"max_retries" mapstructure:"max_retries"`
	// HostedClusterIDs 仅检查这些 clusterID（白名单）；为空表示检查全部系统集群。
	HostedClusterIDs []uint64 `json:"hosted_cluster_ids" yaml:"hosted_cluster_ids" mapstructure:"hosted_cluster_ids"`
}

// DefaultHealthCheckConfig 默认健康检查配置
func DefaultHealthCheckConfig() HealthCheckConfig {
	return HealthCheckConfig{
		Enabled:    true,
		Interval:   30 * time.Second,
		Timeout:    5 * time.Second,
		MaxRetries: 3,
	}
}

// NewHealthChecker 创建健康检查器.
// emitter 可选；未提供时只记录日志和指标，不发送事件。
func NewHealthChecker(
	ctx context.Context,
	cluster *raft.Cluster,
	config HealthCheckConfig,
	emitter ...HealthEventEmitter,
) *HealthChecker {
	ctx, cancel := context.WithCancel(ctx)

	var eventEmitter HealthEventEmitter
	if len(emitter) > 0 {
		eventEmitter = emitter[0]
	}

	hc := &HealthChecker{
		ctx:              ctx,
		cancel:           cancel,
		cluster:          cluster,
		clients:          make(map[uint64]*raft.Client),
		statuses:         make(map[uint64]*ClusterHealthInfo),
		systemClusterIDs: make([]uint64, 0, len(raft.SystemClusterIDs())),
		interval:         config.Interval,
		timeout:          config.Timeout,
		maxRetries:       config.MaxRetries,
		enabled:          config.Enabled,
		emitter:          eventEmitter,
	}

	for _, descriptor := range raft.SystemClusterDescriptors() {
		if !shouldHealthCheckCluster(descriptor.ClusterID, config.HostedClusterIDs) {
			continue
		}
		hc.addCluster(descriptor)
		hc.systemClusterIDs = append(hc.systemClusterIDs, descriptor.ClusterID)
	}

	return hc
}

func (hc *HealthChecker) addCluster(descriptor raft.ClusterDescriptor) {
	client := raft.NewClient(descriptor.ClusterID, hc.cluster, raft.WithTimeout(hc.timeout, hc.timeout))
	hc.clients[descriptor.ClusterID] = client
	hc.statuses[descriptor.ClusterID] = &ClusterHealthInfo{
		ClusterID:   descriptor.ClusterID,
		ClusterName: descriptor.Name,
		Status:      HealthStatusUnknown,
		LastCheck:   time.Now(),
	}
}

// Start 启动健康检查
func (hc *HealthChecker) Start() {
	if !hc.enabled {
		logger.Logger.Info().Msg("Health checker is disabled")
		return
	}

	logger.Logger.Info().
		Dur("interval", hc.interval).
		Dur("timeout", hc.timeout).
		Int("max_retries", hc.maxRetries).
		Msg("Starting cluster health checker")

	go hc.run()
}

// Stop 停止健康检查
func (hc *HealthChecker) Stop() {
	if hc.cancel != nil {
		hc.cancel()
	}
}

// GetHealthStatus 获取集群健康状态
func (hc *HealthChecker) GetHealthStatus(clusterID uint64) (*ClusterHealthInfo, bool) {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()

	status, exists := hc.statuses[clusterID]
	if !exists {
		return nil, false
	}

	// 返回副本以避免并发修改
	statusCopy := *status
	return &statusCopy, true
}

// GetAllHealthStatus 获取所有集群健康状态
func (hc *HealthChecker) GetAllHealthStatus() map[uint64]*ClusterHealthInfo {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()

	result := make(map[uint64]*ClusterHealthInfo)
	for clusterID, status := range hc.statuses {
		statusCopy := *status
		result[clusterID] = &statusCopy
	}

	return result
}

// run 运行健康检查循环
func (hc *HealthChecker) run() {
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	// 立即执行一次检查
	hc.checkAllClusters()

	for {
		select {
		case <-ticker.C:
			hc.checkAllClusters()
		case <-hc.ctx.Done():
			logger.Logger.Info().Msg("Health checker stopped")
			return
		}
	}
}

// checkAllClusters 检查所有集群
func (hc *HealthChecker) checkAllClusters() {
	var wg sync.WaitGroup

	clusterIDs := hc.nextClusterIDsForCheck()
	for _, clusterID := range clusterIDs {
		client := hc.clients[clusterID]
		if client == nil {
			continue
		}
		wg.Add(1)
		go func(cid uint64, c *raft.Client) {
			defer wg.Done()
			hc.checkCluster(cid, c)
		}(clusterID, client)
	}

	wg.Wait()
}

func (hc *HealthChecker) nextClusterIDsForCheck() []uint64 {
	clusterIDs := make([]uint64, 0, len(hc.systemClusterIDs))
	clusterIDs = append(clusterIDs, hc.systemClusterIDs...)
	return clusterIDs
}

func shouldHealthCheckCluster(clusterID uint64, hostedClusterIDs []uint64) bool {
	if len(hostedClusterIDs) == 0 {
		return true
	}
	for _, hostedClusterID := range hostedClusterIDs {
		if hostedClusterID == clusterID {
			return true
		}
	}
	return false
}

// checkCluster 检查单个集群
func (hc *HealthChecker) checkCluster(clusterID uint64, client *raft.Client) {
	startTime := time.Now()

	hc.mutex.Lock()
	status := hc.statuses[clusterID]
	previousStatus := status.Status
	status.LastCheck = startTime
	hc.mutex.Unlock()

	// 执行健康检查
	err := hc.performHealthCheck(client)
	latency := time.Since(startTime)

	hc.mutex.Lock()
	status.Latency = latency

	if err != nil {
		// 检查失败
		status.FailureCount++
		status.Error = err.Error()

		if status.FailureCount >= hc.maxRetries {
			status.Status = HealthStatusUnhealthy
		}

		logger.Logger.Warn().
			Str("cluster", status.ClusterName).
			Uint64("cluster_id", clusterID).
			Err(err).
			Int("failure_count", status.FailureCount).
			Dur("latency", latency).
			Msg("Cluster health check failed")
	} else {
		// 检查成功
		lastSuccessBefore := status.LastSuccess
		wasUnhealthy := status.Status == HealthStatusUnhealthy
		status.Status = HealthStatusHealthy
		status.LastSuccess = startTime
		status.FailureCount = 0
		status.Error = ""

		logger.Logger.Debug().
			Str("cluster", status.ClusterName).
			Uint64("cluster_id", clusterID).
			Dur("latency", latency).
			Msg("Cluster health check succeeded")

		// 如果从不健康恢复到健康，发送恢复事件
		if wasUnhealthy {
			downtime := time.Duration(0)
			if !lastSuccessBefore.IsZero() && startTime.After(lastSuccessBefore) {
				downtime = startTime.Sub(lastSuccessBefore)
			}
			hc.mutex.Unlock()
			hc.emitRecoveryEvent(clusterID, status.ClusterName, downtime)
			hc.mutex.Lock()
		}
	}

	currentStatus := status.Status
	clusterName := status.ClusterName
	hc.mutex.Unlock()

	hc.recordHealthMetrics(clusterID, clusterName, currentStatus, latency)

	// 发送状态变化事件
	hc.emitStatusEvent(clusterID, previousStatus, currentStatus, err)
}

func (hc *HealthChecker) recordHealthMetrics(
	clusterID uint64,
	clusterName string,
	status HealthStatus,
	latency time.Duration,
) {
	statusValue := healthStatusMetricValue(status)
	metric.RecordClusterHealth(clusterName, clusterID, statusValue, latency)
}

func healthStatusMetricValue(status HealthStatus) float64 {
	switch status {
	case HealthStatusHealthy:
		return 1
	case HealthStatusUnhealthy:
		return 0
	default:
		return -1
	}
}

// performHealthCheck 执行实际的健康检查
func (hc *HealthChecker) performHealthCheck(client *raft.Client) error {
	ctx, cancel := context.WithTimeout(hc.ctx, hc.timeout)
	defer cancel()

	_, err := client.Read(ctx, healthcheck.HealthCheckMessage)
	return err
}

func (hc *HealthChecker) emit(eventName string, payload interface{}) {
	if hc == nil || hc.emitter == nil {
		return
	}
	hc.emitter.Emit(eventName, payload)
}

// emitStatusEvent 发送状态事件
func (hc *HealthChecker) emitStatusEvent(clusterID uint64, previousStatus, currentStatus HealthStatus, err error) {
	payload := HealthStatusEvent{
		ClusterID:      clusterID,
		ClusterName:    raft.ClusterName(clusterID),
		PreviousStatus: previousStatus,
		CurrentStatus:  currentStatus,
		Timestamp:      time.Now(),
	}
	if err != nil {
		payload.Error = err.Error()
	}

	// 只在状态发生变化时发送事件
	if previousStatus != currentStatus {
		switch currentStatus {
		case HealthStatusHealthy:
			// 从不健康恢复到健康的事件由 emitRecoveryEvent 单独发送，避免重复事件。
			if previousStatus != HealthStatusUnhealthy {
				// 首次变为健康
				hc.emit(HealthCheckSuccessEvent, payload)
			}
		case HealthStatusUnhealthy:
			hc.emit(HealthCheckFailureEvent, payload)
		}
	}
}

// emitRecoveryEvent 发送恢复事件
func (hc *HealthChecker) emitRecoveryEvent(clusterID uint64, clusterName string, downtime time.Duration) {
	hc.emit(HealthCheckRecoveryEvent, HealthRecoveryEvent{
		ClusterID:    clusterID,
		ClusterName:  clusterName,
		RecoveryTime: time.Now(),
		Downtime:     downtime,
	})

	logger.Logger.Info().
		Str("cluster", clusterName).
		Uint64("cluster_id", clusterID).
		Dur("downtime", downtime).
		Msg("Cluster recovered from failure")
}
