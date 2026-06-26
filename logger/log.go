package logger

import (
	"io"
	stdlog "log"
	"os"
	"path/filepath"
	"time"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/pkg"
	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

var Logger = newDefaultLogger()

// DefaultCtxHook 默认的上下文Hook
// 自动添加客户端ID和上下文ID到日志中
type DefaultCtxHook struct{}

// Run 实现zerolog.Hook接口
func (c *DefaultCtxHook) Run(e *zerolog.Event, level zerolog.Level, message string) {
	ctx := e.GetCtx()
	if ctx != nil {
		// 添加客户端ID
		clientID := pkg.GetClientID(ctx)
		if clientID != "" {
			e.Str("client_id", clientID)
		}

		// 添加上下文ID
		ctxID := pkg.GetContextID(ctx)
		if ctxID != "" {
			e.Str("context_id", ctxID)
		}
	}
}

type SkyLogger struct {
	zerolog.Logger
	config config.Log
	hook   zerolog.Hook
}

func newDefaultLogger() *SkyLogger {
	return &SkyLogger{
		Logger: zerolog.Nop(),
		config: defaultLogConfig(),
	}
}

func defaultLogConfig() config.Log {
	return config.Log{
		Level:            "info",
		File:             "",
		MaxSize:          100,
		MaxAge:           30,
		MaxBackups:       10,
		Compress:         true,
		GinMode:          "release",
		GinConsoleOutput: false,
		StartupReport:    true,
	}
}

// LoadForTest 为测试环境加载日志系统
func LoadForTest() {
	LoadWithHook(nil, defaultLogConfig())
}

// Load 加载日志系统（使用默认hook）
func Load() {
	LoadWithHook(&DefaultCtxHook{}, defaultLogConfig())
}

// LoadWithHook 使用指定的hook加载日志系统
func LoadWithHook(hook zerolog.Hook, cfg config.Log) {
	if cfg.Level == "" {
		cfg = defaultLogConfig()
	}

	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		defaultCfg := defaultLogConfig()
		level, _ = zerolog.ParseLevel(defaultCfg.Level)
		stdlog.Printf("logger: invalid level %q, fallback to %q: %v", cfg.Level, defaultCfg.Level, err)
		cfg.Level = defaultCfg.Level
	}

	// 设置全局日志级别
	zerolog.SetGlobalLevel(level)
	zerolog.TimeFieldFormat = time.RFC3339Nano

	// 创建日志输出器
	consoleOutput := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339Nano,
	}
	output := io.Writer(consoleOutput)
	if cfg.File != "" {
		// 确保日志目录存在
		logDir := filepath.Dir(cfg.File)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			stdlog.Printf("logger: create log dir failed, fallback to console only: dir=%q err=%v", logDir, err)
		} else {
			// 创建文件轮转器
			fileRotator := &lumberjack.Logger{
				Filename:   cfg.File,
				MaxSize:    cfg.MaxSize, // MB
				MaxAge:     cfg.MaxAge,  // 天
				MaxBackups: cfg.MaxBackups,
				Compress:   cfg.Compress,
			}

			// 同时输出到文件和控制台
			output = io.MultiWriter(consoleOutput, fileRotator)
		}
	}

	logger := zerolog.New(output).With().Timestamp().Logger()

	// 如果提供了hook，则应用它
	if hook != nil {
		logger = logger.Hook(hook)
	}

	Logger = &SkyLogger{
		Logger: logger,
		config: cfg,
		hook:   hook,
	}
}

// GetConfig 获取当前日志配置
func (l *SkyLogger) GetConfig() config.Log {
	return l.config
}

// Reload 重新加载日志配置
func (l *SkyLogger) Reload() {
	LoadWithHook(l.hook, l.config)
	// Ensure Dragonboat log level is synced after reload.
	SyncDragonboatLogLevelFromSkyLogger()
}

// ReloadWithHook 使用新的hook重新加载日志配置
func (l *SkyLogger) ReloadWithHook(hook zerolog.Hook) {
	LoadWithHook(hook, l.config)
	// Ensure Dragonboat log level is synced after reload.
	SyncDragonboatLogLevelFromSkyLogger()
}

// SetLevel 动态设置日志级别
func (l *SkyLogger) SetLevel(level string) error {
	logLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		return err
	}
	zerolog.SetGlobalLevel(logLevel)
	// Keep Dragonboat log level in sync with SkyTree's runtime updates.
	SyncDragonboatLogLevelFromSkyLogger()
	return nil
}

// GetHook 获取当前使用的hook
func (l *SkyLogger) GetHook() zerolog.Hook {
	return l.hook
}
