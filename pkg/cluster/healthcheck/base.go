package healthcheck

import (
	"io"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	"github.com/lni/dragonboat/v3/statemachine"
)

const (
	// HealthCheckMessage 健康检查消息标识
	HealthCheckMessage = "health_check"
	// DefaultRequestTimeout 默认请求超时时间
	DefaultRequestTimeout = 5 * time.Second
)

// HealthCheckWrapper 健康检查包装器，只处理健康检查逻辑
type HealthCheckWrapper struct {
	// 原始状态机
	original statemachine.IStateMachine
}

// NewHealthCheckWrapper 创建健康检查包装器
func NewHealthCheckWrapper(original statemachine.IStateMachine) *HealthCheckWrapper {
	return &HealthCheckWrapper{
		original: original,
	}
}

// Update 处理Update请求，自动处理健康检查
func (w *HealthCheckWrapper) Update(bytes []byte) (statemachine.Result, error) {
	if len(bytes) == 0 {
		logger.Logger.Warn().Msg("Empty update data")
		return statemachine.Result{}, nil
	}

	// 健康检查处理
	if string(bytes) == HealthCheckMessage {
		logger.Logger.Debug().Msg("health check request received")
		return statemachine.Result{}, nil
	}

	// 其他请求直接委托给原始状态机
	return w.original.Update(bytes)
}

// Lookup 直接委托给原始状态机
func (w *HealthCheckWrapper) Lookup(i interface{}) (interface{}, error) {
	if msg, ok := i.(string); ok && msg == HealthCheckMessage {
		logger.Logger.Debug().Msg("health check query received")
		return true, nil
	}
	return w.original.Lookup(i)
}

// SaveSnapshot 直接委托给原始状态机
func (w *HealthCheckWrapper) SaveSnapshot(writer io.Writer, collection statemachine.ISnapshotFileCollection, i <-chan struct{}) error {
	return w.original.SaveSnapshot(writer, collection, i)
}

// RecoverFromSnapshot 直接委托给原始状态机
func (w *HealthCheckWrapper) RecoverFromSnapshot(reader io.Reader, files []statemachine.SnapshotFile, i <-chan struct{}) error {
	return w.original.RecoverFromSnapshot(reader, files, i)
}

// Close 直接委托给原始状态机
func (w *HealthCheckWrapper) Close() error {
	return w.original.Close()
}
