package lifecycle

import (
	"context"
	"time"
)

const DefaultCriticalStartupGrace = 500 * time.Millisecond

// Component 表示可由 App 生命周期统一托管的运行组件。
type Component interface {
	Start(ctx context.Context) error
	Close() error
	Name() string
}

// StartMode 描述 runner 判断组件 ready 的方式。
type StartMode int

const (
	// StartModeBlocking 表示组件 Start 会长期阻塞，runner 通过宽限期判断 ready。
	StartModeBlocking StartMode = iota
	// StartModeOneShot 表示组件 Start 是一次性初始化，返回 nil 即 ready。
	StartModeOneShot
)

// ManagedComponent 保存组件实例及其启动策略。
type ManagedComponent struct {
	Component    Component
	Critical     bool
	StartMode    StartMode
	StartupGrace time.Duration
}

// ReadyResult 表示关键组件 ready 检查结果。
type ReadyResult struct {
	Component string
	Err       error
}
