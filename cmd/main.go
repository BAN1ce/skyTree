package main

import (
	"context"
	app2 "github.com/BAN1ce/skyTree/inner/app"
	"github.com/BAN1ce/skyTree/logger"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	var (
		app         = app2.NewApp()
		ctx, cancel = context.WithCancel(context.Background())
		err         = app.Start(ctx)
	)
	if err != nil {
		log.Fatalln("start app error: ", err)
	}
	logger.Logger.Info("start app success")
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-quit
	cancel()
	time.Sleep(1 * time.Second)
	logger.Logger.Info("stop app success")
	os.Exit(0)
}
