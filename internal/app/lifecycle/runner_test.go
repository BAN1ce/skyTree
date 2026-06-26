package lifecycle

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/BAN1ce/skyTree/logger"
	"github.com/rs/zerolog"
)

// testComponent 用于模拟 runner 测试中的托管组件。
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

// ensureTestLogger 确保 runner 测试具备可用 logger。
func ensureTestLogger() {
	if logger.Logger != nil {
		return
	}
	logger.Logger = &logger.SkyLogger{
		Logger: zerolog.New(io.Discard),
	}
}

// TestRunnerStartWaitsForCriticalReadinessWindow 验证阻塞型关键组件会等待 ready 窗口。
func TestRunnerStartWaitsForCriticalReadinessWindow(t *testing.T) {
	ensureTestLogger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner := NewRunner(ctx, []ManagedComponent{
		{
			Component: testComponent{
				name: "blocking-critical",
				startFn: func(ctx context.Context) error {
					<-ctx.Done()
					return nil
				},
			},
			Critical:     true,
			StartMode:    StartModeBlocking,
			StartupGrace: 80 * time.Millisecond,
		},
	})

	startedAt := time.Now()
	if err := runner.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	elapsed := time.Since(startedAt)
	if elapsed < 80*time.Millisecond {
		t.Fatalf("start returned before readiness window: %v", elapsed)
	}

	if err := runner.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}

// TestRunnerStartFailsWhenCriticalComponentFails 验证关键组件启动失败会返回错误。
func TestRunnerStartFailsWhenCriticalComponentFails(t *testing.T) {
	ensureTestLogger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner := NewRunner(ctx, []ManagedComponent{
		{
			Component: testComponent{
				name: "critical-fail",
				startFn: func(context.Context) error {
					return errors.New("boom")
				},
			},
			Critical:  true,
			StartMode: StartModeOneShot,
		},
	})

	err := runner.Start()
	if err == nil {
		t.Fatal("expected startup error, got nil")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("unexpected startup error: %v", err)
	}
}

// TestRunnerRunErrorsReportsBackgroundComponentError 验证后台组件异常会进入运行错误通道。
func TestRunnerRunErrorsReportsBackgroundComponentError(t *testing.T) {
	ensureTestLogger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner := NewRunner(ctx, []ManagedComponent{
		{
			Component: testComponent{
				name: "critical-blocking",
				startFn: func(ctx context.Context) error {
					<-ctx.Done()
					return nil
				},
			},
			Critical:     true,
			StartMode:    StartModeBlocking,
			StartupGrace: 10 * time.Millisecond,
		},
		{
			Component: testComponent{
				name: "background",
				startFn: func(context.Context) error {
					time.Sleep(20 * time.Millisecond)
					return errors.New("background failed")
				},
			},
			Critical:  false,
			StartMode: StartModeBlocking,
		},
	})

	if err := runner.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	select {
	case err := <-runner.RunErrors():
		if err == nil || !strings.Contains(err.Error(), "background failed") {
			t.Fatalf("unexpected run error: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for background error")
	}

	if err := runner.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}

// TestRunnerCloseIsIdempotent 验证 runner 关闭操作可重复调用。
func TestRunnerCloseIsIdempotent(t *testing.T) {
	ensureTestLogger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner := NewRunner(ctx, nil)

	if err := runner.Close(); err != nil {
		t.Fatalf("first close failed: %v", err)
	}
	if err := runner.Close(); err != nil {
		t.Fatalf("second close failed: %v", err)
	}
}

// TestRunnerCloseComponentsInReverse 验证组件按注册顺序反向关闭。
func TestRunnerCloseComponentsInReverse(t *testing.T) {
	ensureTestLogger()

	var closed []string
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runner := NewRunner(ctx, []ManagedComponent{
		{
			Component: testComponent{
				name:    "first",
				startFn: func(context.Context) error { return nil },
				closeFn: func() error {
					closed = append(closed, "first")
					return nil
				},
			},
			StartMode: StartModeOneShot,
		},
		{
			Component: testComponent{
				name:    "second",
				startFn: func(context.Context) error { return nil },
				closeFn: func() error {
					closed = append(closed, "second")
					return nil
				},
			},
			StartMode: StartModeOneShot,
		},
	})

	if err := runner.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if got, want := strings.Join(closed, ","), "second,first"; got != want {
		t.Fatalf("unexpected close order: got %q want %q", got, want)
	}
}
