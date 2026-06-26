package app

import (
	"errors"
	"strings"
	"time"

	"github.com/BAN1ce/skyTree/internal/app/lifecycle"
	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/broker/plugin"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/kataras/go-events"
	"github.com/rs/zerolog"
)

// loggerInitFn 描述日志初始化函数，便于测试替换默认 logger 初始化逻辑。
type loggerInitFn func(hook zerolog.Hook, cfg config.Log)

// eventDriverBinderFn 描述事件总线绑定函数，便于外部注入事件监听器。
type eventDriverBinderFn func(driver events.EventEmmiter)

// Option 表示 NewApp 构建阶段可选配置项。
type Option func(*Config)

// Config 保存应用构建阶段需要的可选依赖和策略。
type Config struct {
	Plugins              *plugin.Plugins
	LogHook              zerolog.Hook
	LoggerInitializer    loggerInitFn
	EventDriverBinder    eventDriverBinderFn
	CriticalStartupGrace time.Duration
}

// defaultConfig 根据应用配置生成默认构建选项。
func defaultConfig(cfg config.AppConfig) *Config {
	return &Config{
		Plugins:              buildDefaultPlugins(cfg),
		LogHook:              &logger.DefaultCtxHook{},
		LoggerInitializer:    logger.LoadWithHook,
		CriticalStartupGrace: lifecycle.DefaultCriticalStartupGrace,
	}
}

// WithPlugins 使用指定插件集合替换默认插件集合。
func WithPlugins(plugins *plugin.Plugins) Option {
	return func(cfg *Config) {
		cfg.Plugins = plugins
	}
}

// WithCustomPlugins 通过插件 builder 生成插件集合。
func WithCustomPlugins(builder *plugin.Builder) Option {
	return func(cfg *Config) {
		cfg.Plugins = builder.Build()
	}
}

// WithLogHook 设置日志 hook。
func WithLogHook(hook zerolog.Hook) Option {
	return func(cfg *Config) {
		cfg.LogHook = hook
	}
}

// WithLoggerInitializer 注入日志初始化函数，主要用于测试和定制启动行为。
func WithLoggerInitializer(initFn func(hook zerolog.Hook, cfg config.Log)) Option {
	return func(cfg *Config) {
		cfg.LoggerInitializer = initFn
	}
}

// WithEventDriverBinder 注入事件总线绑定函数。
func WithEventDriverBinder(bindFn func(driver events.EventEmmiter)) Option {
	return func(cfg *Config) {
		cfg.EventDriverBinder = bindFn
	}
}

// WithCriticalStartupGrace 设置关键阻塞组件的启动 ready 宽限期。
func WithCriticalStartupGrace(grace time.Duration) Option {
	return func(cfg *Config) {
		cfg.CriticalStartupGrace = grace
	}
}

// initLogger 执行日志初始化并校验全局 logger 已可用。
func initLogger(appConfig *Config, logCfg config.Log) error {
	if appConfig.LoggerInitializer == nil {
		return errors.New("logger initializer is nil")
	}
	appConfig.LoggerInitializer(appConfig.LogHook, logCfg)
	if logger.Logger == nil {
		return errors.New("logger initializer completed but logger is nil")
	}
	return nil
}

// buildDefaultPlugins 根据配置启用默认插件集合。
func buildDefaultPlugins(cfg config.AppConfig) *plugin.Plugins {
	builder := plugin.NewBuilder()

	if cfg.Plugins.Metric.Enabled {
		builder.AddMetric()
	}

	if cfg.Plugins.Auth.Enabled {
		builder.AddAuth()
	}

	if cfg.Plugins.Auth.Auth.Enabled {
		builder.AddAuthPacket(cfg.Plugins.Auth.Auth)
	}

	builder.AddErrorHandler()

	return builder.Build()
}

// getPluginStatus 返回用于启动日志展示的插件状态摘要。
func getPluginStatus(plugins *plugin.Plugins) string {
	if plugins == nil {
		return "none"
	}

	status := make([]string, 0, 3)
	if len(plugins.OnReceivedConnect) > 0 {
		status = append(status, "connect")
	}
	if len(plugins.OnSubscribe) > 0 {
		status = append(status, "subscribe")
	}
	if len(plugins.OnReceivedPublish) > 0 {
		status = append(status, "publish")
	}

	if len(status) == 0 {
		return "none"
	}

	return "[" + strings.Join(status, ",") + "]"
}
