package main

import (
	"context"
	app2 "github.com/BAN1ce/skyTree/inner/app"
	"github.com/BAN1ce/skyTree/logger"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// @title           SkyTree API
// @version         1.0

// @host      localhost:8080
// @BasePath  /api/v1

// @securityDefinitions.basic  BasicAuth
func main() {
	var (
		app         = app2.NewApp()
		ctx, cancel = context.WithCancel(context.Background())
	)
	app.Start(ctx)
	logger.Logger.Info("start app success")
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-quit
	cancel()
	time.Sleep(1 * time.Second)
	logger.Logger.Info("stop app success")
	os.Exit(0)
}
