package cmdruntime

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	app2 "github.com/BAN1ce/skyTree/internal/app"
	"github.com/BAN1ce/skyTree/logger"
)

type ShutdownController struct {
	graceTimeout time.Duration
}

func NewShutdownController(graceTimeout time.Duration) *ShutdownController {
	return &ShutdownController{graceTimeout: graceTimeout}
}

func (c *ShutdownController) Wait(app *app2.App) {
	if app == nil {
		return
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case sig := <-quit:
		logger.Logger.Info().Str("signal", sig.String()).Msg("received shutdown signal")
	case err := <-app.RunErrors():
		if err != nil {
			logger.Logger.Error().Err(err).Msg("application component exited unexpectedly")
		} else {
			logger.Logger.Info().Msg("application components stopped")
		}
	}
}

func (c *ShutdownController) Close(app *app2.App, cancel func()) {
	if app == nil {
		return
	}

	exitTimeout(c.graceTimeout)
	if cancel != nil {
		cancel()
	}

	if err := app.Close(); err != nil {
		logger.Logger.Error().Err(err).Msg("app close error")
		return
	}
	logger.Logger.Info().Msg("exit successfully")
}

func exitTimeout(t time.Duration) {
	go func() {
		time.Sleep(t)
		logger.Logger.Error().Msg("exit timeout")
		os.Exit(1)
	}()
}

func ExitWithError(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
