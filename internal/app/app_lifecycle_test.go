package app

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/BAN1ce/skyTree/internal/app/bootstrap"
	"github.com/BAN1ce/skyTree/internal/app/lifecycle"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/rs/zerolog"
)

// testComponent 用于模拟 App 生命周期测试中的托管组件。
type testComponent struct {
	name    string
	startFn func(ctx context.Context) error
	closeFn func() error
}

// Start 执行测试注入的启动逻辑。
func (c testComponent) Start(ctx context.Context) error {
	return c.startFn(ctx)
}

// Name 返回测试组件名称。
func (c testComponent) Name() string {
	return c.name
}

// Close 执行测试注入的关闭逻辑。
func (c testComponent) Close() error {
	if c.closeFn != nil {
		return c.closeFn()
	}
	return nil
}

// ensureTestLogger 确保生命周期测试具备可用 logger。
func ensureTestLogger() {
	if logger.Logger != nil {
		return
	}
	logger.Logger = &logger.SkyLogger{
		Logger: zerolog.New(io.Discard),
	}
}

// newLifecycleTestApp 创建只包含 lifecycle runner 的测试 App。
func newLifecycleTestApp(components []lifecycle.ManagedComponent) *App {
	ctx, cancel := context.WithCancel(context.Background())
	return &App{
		ctx:    ctx,
		cancel: cancel,
		runtime: &bootstrap.AppRuntime{
			Runner: lifecycle.NewRunner(ctx, components),
		},
	}
}

// TestAppStartFailsWhenRuntimeComponentFails 验证关键组件失败会导致 App 启动失败。
func TestAppStartFailsWhenRuntimeComponentFails(t *testing.T) {
	ensureTestLogger()

	app := newLifecycleTestApp([]lifecycle.ManagedComponent{
		{
			Component: testComponent{
				name: "critical-fail",
				startFn: func(context.Context) error {
					return errors.New("boom")
				},
			},
			Critical:  true,
			StartMode: lifecycle.StartModeOneShot,
		},
	})

	err := app.Start()
	if err == nil {
		t.Fatal("expected startup error, got nil")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected startup error: %v", err)
	}
}

// TestAppCloseIsIdempotent 验证 App 关闭操作可重复调用。
func TestAppCloseIsIdempotent(t *testing.T) {
	ensureTestLogger()

	app := newLifecycleTestApp(nil)
	if err := app.Close(); err != nil {
		t.Fatalf("first close failed: %v", err)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("second close failed: %v", err)
	}
}

// TestAppStartAfterCloseReturnsError 验证关闭后的 App 不能再次启动。
func TestAppStartAfterCloseReturnsError(t *testing.T) {
	ensureTestLogger()

	app := newLifecycleTestApp(nil)
	if err := app.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	err := app.Start()
	if err == nil {
		t.Fatal("expected start after close to fail")
	}
	if !strings.Contains(err.Error(), "app already closed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestAppRunErrorsIsClosedAfterClose 验证关闭后运行错误通道处于关闭状态。
func TestAppRunErrorsIsClosedAfterClose(t *testing.T) {
	ensureTestLogger()

	app := newLifecycleTestApp(nil)
	if err := app.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	select {
	case _, ok := <-app.RunErrors():
		if ok {
			t.Fatal("expected closed run error channel")
		}
	default:
		t.Fatal("expected run error channel to be closed")
	}
}
