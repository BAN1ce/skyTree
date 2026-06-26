package app

import (
	"errors"
	"fmt"

	"github.com/BAN1ce/skyTree/logger"
)

// Start 启动应用运行时中的托管组件，并在关键组件 ready 后返回。
func (a *App) Start() error {
	a.mux.Lock()
	if a.closed {
		a.mux.Unlock()
		return fmt.Errorf("app already closed")
	}
	if a.started {
		a.mux.Unlock()
		return fmt.Errorf("app already started")
	}
	a.started = true
	appRuntime := a.runtime
	a.mux.Unlock()

	if appRuntime == nil || appRuntime.Runner == nil {
		return fmt.Errorf("app runtime is nil")
	}

	if err := appRuntime.Runner.Start(); err != nil {
		closeErr := a.Close()
		if closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	if appRuntime.HealthChecker != nil {
		appRuntime.HealthChecker.Start()
	}
	return nil
}

// RunErrors 返回后台组件运行期错误通道，调用方可用它感知异步退出。
func (a *App) RunErrors() <-chan error {
	a.mux.Lock()
	defer a.mux.Unlock()
	if a.runtime == nil || a.runtime.Runner == nil {
		ch := make(chan error)
		close(ch)
		return ch
	}
	return a.runtime.Runner.RunErrors()
}

// Close 按应用生命周期顺序停止健康检查、组件 runner 和底层资源。
func (a *App) Close() error {
	a.mux.Lock()
	if a.closed {
		a.mux.Unlock()
		return nil
	}

	cancel := a.cancel
	a.cancel = nil
	appRuntime := a.runtime
	a.runtime = nil
	a.started = false
	a.closed = true
	a.mux.Unlock()

	if cancel != nil {
		cancel()
	}

	var closeErr error
	if appRuntime != nil {
		if appRuntime.HealthChecker != nil {
			appRuntime.HealthChecker.Stop()
		}
		if appRuntime.Runner != nil {
			closeErr = errors.Join(closeErr, appRuntime.Runner.Close())
		}
		closeErr = errors.Join(closeErr, appRuntime.CloseResources())
	}

	if closeErr != nil {
		logger.Logger.Error().Err(closeErr).Msg("close app with errors")
		return closeErr
	}

	logger.Logger.Info().Msg("all closes executed")
	return nil
}
