package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/BAN1ce/skyTree/logger"
)

// Runner 负责托管组件的启动、运行错误上报和关闭顺序。
type Runner struct {
	ctx        context.Context
	cancel     context.CancelFunc
	components []ManagedComponent

	mux      sync.Mutex
	wg       sync.WaitGroup
	runErrCh chan error
	started  bool
	closed   bool
}

// NewRunner 创建一个绑定父 context 的生命周期 runner。
func NewRunner(parent context.Context, components []ManagedComponent) *Runner {
	ctx, cancel := context.WithCancel(parent)
	return &Runner{
		ctx:        ctx,
		cancel:     cancel,
		components: append([]ManagedComponent(nil), components...),
	}
}

// Start 启动所有托管组件，并等待关键组件达到 ready 状态。
func (r *Runner) Start() error {
	r.mux.Lock()
	if r.closed {
		r.mux.Unlock()
		return fmt.Errorf("runner already closed")
	}
	if r.started {
		r.mux.Unlock()
		return fmt.Errorf("runner already started")
	}
	r.started = true
	r.runErrCh = make(chan error, len(r.components)+1)
	components := append([]ManagedComponent(nil), r.components...)
	r.mux.Unlock()

	criticalCount := countCriticalComponents(components)
	readyCh := make(chan ReadyResult, criticalCount)
	startBegin := time.Now()
	for _, component := range components {
		r.wg.Add(1)
		go r.runComponent(component, readyCh)
	}

	for i := 0; i < criticalCount; i++ {
		ready := <-readyCh
		if ready.Err == nil {
			continue
		}
		closeErr := r.Close()
		if closeErr != nil {
			return errors.Join(ready.Err, closeErr)
		}
		return ready.Err
	}

	logger.Logger.Info().
		Str("phase", "start_components").
		Int64("duration_ms", time.Since(startBegin).Milliseconds()).
		Str("result", "ok").
		Msg("critical components are ready")

	return nil
}

// countCriticalComponents 统计会阻塞 Start 成功返回的关键组件数量。
func countCriticalComponents(components []ManagedComponent) int {
	var count int
	for _, component := range components {
		if component.Critical {
			count++
		}
	}
	return count
}

// runComponent 启动单个组件，并在运行期异常退出时上报错误。
func (r *Runner) runComponent(component ManagedComponent, readyCh chan<- ReadyResult) {
	defer r.wg.Done()

	name := component.Component.Name()
	startAt := time.Now()
	doneCh := make(chan error, 1)
	go func() {
		doneCh <- component.Component.Start(r.ctx)
	}()

	sendReady := func(err error) {
		if !component.Critical {
			return
		}
		readyCh <- ReadyResult{Component: name, Err: err}
	}

	if component.Critical {
		ready, shouldReturn := r.waitForCriticalReady(component, name, startAt, doneCh)
		sendReady(ready)
		if shouldReturn {
			return
		}
	}

	err := <-doneCh
	if expectedShutdown(r.ctx, err) {
		logger.Logger.Info().
			Str("phase", "component_runtime").
			Str("component", name).
			Str("result", "stopped").
			Int64("duration_ms", time.Since(startAt).Milliseconds()).
			Msg("component stopped")
		return
	}
	if err != nil {
		r.reportRunError(name, err)
		return
	}
	if component.StartMode == StartModeBlocking {
		r.reportRunError(name, fmt.Errorf("component exited without error"))
		return
	}

	logger.Logger.Info().
		Str("phase", "component_runtime").
		Str("component", name).
		Str("result", "completed").
		Int64("duration_ms", time.Since(startAt).Milliseconds()).
		Msg("component completed")
}

// waitForCriticalReady 根据组件启动模式等待关键组件 ready。
func (r *Runner) waitForCriticalReady(
	component ManagedComponent,
	name string,
	startAt time.Time,
	doneCh <-chan error,
) (error, bool) {
	switch component.StartMode {
	case StartModeOneShot:
		err := <-doneCh
		if err != nil {
			return fmt.Errorf("critical component %s start failed: %w", name, err), true
		}
		logger.Logger.Info().
			Str("phase", "component_start").
			Str("component", name).
			Str("result", "ready").
			Int64("duration_ms", time.Since(startAt).Milliseconds()).
			Msg("component started")
		return nil, true
	case StartModeBlocking:
		return r.waitForBlockingCritical(component, name, startAt, doneCh)
	default:
		return fmt.Errorf("component %s has unknown start mode", name), true
	}
}

// waitForBlockingCritical 通过宽限期判断阻塞型关键组件是否 ready。
func (r *Runner) waitForBlockingCritical(
	component ManagedComponent,
	name string,
	startAt time.Time,
	doneCh <-chan error,
) (error, bool) {
	grace := component.StartupGrace
	if grace <= 0 {
		grace = DefaultCriticalStartupGrace
	}
	timer := time.NewTimer(grace)
	defer stopTimer(timer)

	select {
	case err := <-doneCh:
		if expectedShutdown(r.ctx, err) {
			return nil, true
		}
		if err != nil {
			return fmt.Errorf("critical component %s start failed: %w", name, err), true
		}
		return fmt.Errorf("critical component %s exited before readiness window", name), true
	case <-timer.C:
		logger.Logger.Info().
			Str("phase", "component_start").
			Str("component", name).
			Str("result", "ready").
			Int64("duration_ms", time.Since(startAt).Milliseconds()).
			Msg("component reached readiness window")
		return nil, false
	case <-r.ctx.Done():
		return nil, true
	}
}

// stopTimer 安全停止 timer，并清理可能残留的触发信号。
func stopTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

// expectedShutdown 判断组件退出是否由正常取消触发。
func expectedShutdown(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return true
	}
	if err == nil {
		return false
	}
	return errors.Is(err, context.Canceled)
}

// reportRunError 将组件运行期异常写入错误通道并记录日志。
func (r *Runner) reportRunError(componentName string, componentErr error) {
	runErr := fmt.Errorf("component %s exited with error: %w", componentName, componentErr)

	r.mux.Lock()
	ch := r.runErrCh
	r.mux.Unlock()

	if ch != nil {
		select {
		case ch <- runErr:
		default:
			logger.Logger.Error().Err(runErr).Str("component", componentName).Msg("app run error channel is full")
		}
	}

	logger.Logger.Error().Err(runErr).Str("component", componentName).Msg("component exited unexpectedly")
}

// RunErrors 返回组件运行期错误通道。
func (r *Runner) RunErrors() <-chan error {
	r.mux.Lock()
	defer r.mux.Unlock()
	return r.runErrCh
}

// Close 取消 runner context，关闭托管组件，并等待组件 goroutine 退出。
func (r *Runner) Close() error {
	r.mux.Lock()
	if r.closed {
		r.mux.Unlock()
		return nil
	}

	cancel := r.cancel
	r.cancel = nil
	components := append([]ManagedComponent(nil), r.components...)
	ch := r.runErrCh
	r.runErrCh = nil
	r.started = false
	r.closed = true
	r.mux.Unlock()

	if cancel != nil {
		cancel()
	}

	closeErr := closeComponentsReverse(components)
	r.wg.Wait()

	closedRunErrCh := ch
	if ch != nil {
		close(ch)
	} else {
		closedRunErrCh = make(chan error)
		close(closedRunErrCh)
	}

	r.mux.Lock()
	if r.runErrCh == nil {
		r.runErrCh = closedRunErrCh
	}
	r.mux.Unlock()

	if closeErr != nil {
		logger.Logger.Error().Err(closeErr).Msg("close components with errors")
		return closeErr
	}

	logger.Logger.Info().Msg("all components closed")
	return nil
}

// closeComponentsReverse 按注册顺序反向关闭组件并聚合错误。
func closeComponentsReverse(components []ManagedComponent) error {
	var err error
	for i := len(components) - 1; i >= 0; i-- {
		component := components[i].Component
		if component == nil {
			continue
		}
		if closeErr := component.Close(); closeErr != nil {
			err = errors.Join(err, closeErr)
		}
	}
	return err
}
