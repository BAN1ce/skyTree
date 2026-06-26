package app

import (
	"context"
	"sync"

	"github.com/BAN1ce/skyTree/internal/app/bootstrap"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/version"
	"github.com/BAN1ce/skyTree/logger"
)

// App 表示 skyTree 进程内的应用实例，只暴露启动、关闭和运行错误读取能力。
type App struct {
	ctx     context.Context
	cancel  context.CancelFunc
	runtime *bootstrap.AppRuntime
	mux     sync.Mutex
	started bool
	closed  bool
}

// NewApp 根据配置构建应用运行时，并完成日志、插件和启动依赖初始化。
func NewApp(parent context.Context, cfg config.AppConfig, opts ...Option) (app *App, err error) {
	appConfig := defaultConfig(cfg)
	for _, opt := range opts {
		opt(appConfig)
	}

	if err = initLogger(appConfig, cfg.Logging); err != nil {
		return nil, err
	}

	logger.Logger.Info().
		Str("phase", "new_app").
		Str("plugin_status", getPluginStatus(appConfig.Plugins)).
		Msg("building app")

	ctx, cancel := context.WithCancel(parent)
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	appRuntime, err := bootstrap.BuildAppRuntime(ctx, cfg, bootstrap.Config{
		Plugins:              appConfig.Plugins,
		EventDriverBinder:    bootstrap.EventDriverBinder(appConfig.EventDriverBinder),
		CriticalStartupGrace: appConfig.CriticalStartupGrace,
	})
	if err != nil {
		return nil, err
	}

	app = &App{
		ctx:     ctx,
		cancel:  cancel,
		runtime: appRuntime,
	}

	logger.Logger.Info().
		Str("phase", "new_app").
		Str("result", "ok").
		Msg("app initialized")

	return app, nil
}

// Version 返回当前应用编译时注入的版本号。
func (a *App) Version() string {
	return version.Version
}
