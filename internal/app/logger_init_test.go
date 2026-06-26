package app

import (
	"testing"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/rs/zerolog"
)

func TestInitLoggerRunsInitializerEvenWhenDefaultLoggerExists(t *testing.T) {
	original := logger.Logger
	t.Cleanup(func() {
		logger.Logger = original
	})

	logger.Logger = &logger.SkyLogger{Logger: zerolog.Nop()}

	logCfg := config.Log{
		Level:      "debug",
		File:       "/tmp/skytree-test.log",
		MaxSize:    8,
		MaxAge:     2,
		MaxBackups: 1,
		Compress:   false,
	}

	var (
		calls   int
		gotHook zerolog.Hook
		gotCfg  config.Log
	)
	hook := &logger.DefaultCtxHook{}
	err := initLogger(&Config{
		LogHook: hook,
		LoggerInitializer: func(h zerolog.Hook, cfg config.Log) {
			calls++
			gotHook = h
			gotCfg = cfg
		},
	}, logCfg)
	if err != nil {
		t.Fatalf("initLogger returned error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("logger initializer calls = %d, want 1", calls)
	}
	if gotHook != hook {
		t.Fatalf("logger initializer hook mismatch")
	}
	if gotCfg != logCfg {
		t.Fatalf("logger initializer cfg mismatch: got %+v want %+v", gotCfg, logCfg)
	}
}
