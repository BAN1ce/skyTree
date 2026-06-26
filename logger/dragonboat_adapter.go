package logger

// Dragonboat logger adapter and log level sync.

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	dblogger "github.com/lni/dragonboat/v3/logger"
	"github.com/rs/zerolog"
)

var dragonboatInitOnce sync.Once

// DragonboatZerologAdapter adapts Dragonboat ILogger to SkyTree zerolog.
// It performs per-logger filtering based on Dragonboat's LogLevel and also respects
// zerolog global level (zerolog.SetGlobalLevel).
type DragonboatZerologAdapter struct {
	pkgName string
	level   atomic.Int32 // stores dblogger.LogLevel as int32
}

var _ dblogger.ILogger = (*DragonboatZerologAdapter)(nil)

func newDragonboatZerologAdapter(pkgName string) *DragonboatZerologAdapter {
	a := &DragonboatZerologAdapter{pkgName: pkgName}
	a.level.Store(int32(mapZerologToDragonboat(zerolog.GlobalLevel())))
	return a
}

func (a *DragonboatZerologAdapter) SetLevel(l dblogger.LogLevel) {
	a.level.Store(int32(l))
}

func (a *DragonboatZerologAdapter) Debugf(format string, args ...interface{}) {
	if !a.allow(dblogger.DEBUG) {
		return
	}
	a.emit(zerolog.DebugLevel, format, args...)
}

func (a *DragonboatZerologAdapter) Infof(format string, args ...interface{}) {
	if !a.allow(dblogger.INFO) {
		return
	}
	a.emit(zerolog.InfoLevel, format, args...)
}

func (a *DragonboatZerologAdapter) Warningf(format string, args ...interface{}) {
	if !a.allow(dblogger.WARNING) {
		return
	}
	a.emit(zerolog.WarnLevel, format, args...)
}

func (a *DragonboatZerologAdapter) Errorf(format string, args ...interface{}) {
	if !a.allow(dblogger.ERROR) {
		return
	}
	a.emit(zerolog.ErrorLevel, format, args...)
}

func (a *DragonboatZerologAdapter) Panicf(format string, args ...interface{}) {
	// Panic is treated as critical to match Dragonboat semantics.
	if !a.allow(dblogger.CRITICAL) {
		return
	}
	a.emit(zerolog.PanicLevel, format, args...)
}

func (a *DragonboatZerologAdapter) allow(msgLevel dblogger.LogLevel) bool {
	// Dragonboat level is a verbosity threshold. With WARNING, it
	// allows WARNING/ERROR/CRITICAL and blocks INFO/DEBUG.
	current := dblogger.LogLevel(a.level.Load())
	return msgLevel <= current
}

func (a *DragonboatZerologAdapter) emit(level zerolog.Level, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)

	// Fallback for early logs before logger.LoadWithHook().
	if Logger == nil {
		log.Printf("[dragonboat][%s][%s] %s", a.pkgName, level.String(), msg)
		return
	}

	evt := Logger.With().Str("component", "dragonboat").Str("dragonboat_pkg", a.pkgName).Logger()

	switch level {
	case zerolog.DebugLevel:
		evt.Debug().Msg(msg)
	case zerolog.InfoLevel:
		evt.Info().Msg(msg)
	case zerolog.WarnLevel:
		evt.Warn().Msg(msg)
	case zerolog.ErrorLevel:
		evt.Error().Msg(msg)
	case zerolog.PanicLevel:
		evt.Panic().Msg(msg)
	case zerolog.FatalLevel:
		evt.Fatal().Msg(msg)
	default:
		evt.Info().Msg(msg)
	}
}

// InitDragonboatLoggerFactory registers Dragonboat logger factory exactly once.
// Dragonboat will panic if SetLoggerFactory is called more than once.
func InitDragonboatLoggerFactory() {
	dragonboatInitOnce.Do(func() {
		dblogger.SetLoggerFactory(func(pkgName string) dblogger.ILogger {
			return newDragonboatZerologAdapter(pkgName)
		})
	})
}

// InjectDragonboatLogger explicitly injects SkyTree logger into Dragonboat.
// Call this once during startup after logger.LoadWithHook() and before creating NodeHost.
func InjectDragonboatLogger() {
	InitDragonboatLoggerFactory()
	SyncDragonboatLogLevelFromSkyLogger()
}

// SyncDragonboatLogLevelFromSkyLogger aligns Dragonboat logger levels with
// current zerolog global level, so runtime logger.Logger.SetLevel(...) changes take effect.
func SyncDragonboatLogLevelFromSkyLogger() {
	level := mapZerologToDragonboat(zerolog.GlobalLevel())
	setDragonboatModulesLevel(level)
}

func mapZerologToDragonboat(l zerolog.Level) dblogger.LogLevel {
	switch l {
	case zerolog.DebugLevel, zerolog.TraceLevel:
		return dblogger.DEBUG
	case zerolog.InfoLevel:
		return dblogger.INFO
	case zerolog.WarnLevel:
		return dblogger.WARNING
	case zerolog.ErrorLevel:
		return dblogger.ERROR
	case zerolog.FatalLevel, zerolog.PanicLevel:
		return dblogger.CRITICAL
	default:
		return dblogger.INFO
	}
}

func setDragonboatModulesLevel(l dblogger.LogLevel) {
	// Common module names used by Dragonboat v3.
	for _, name := range []string{
		"dragonboat",
		"raft",
		"raft-mt",
		"raftpb",
		"rsm",
		"logdb",
		"LogDB",
		"transport",
		"server",
		"settings",
		"config",
		"pebblekv",
		"rocksdb",
		"tools",
	} {
		dblogger.GetLogger(name).SetLevel(l)
	}
}
