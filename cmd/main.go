package main

import (
	"context"
	"flag"
	"fmt"
	app2 "github.com/BAN1ce/skyTree/app"
	"github.com/BAN1ce/skyTree/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"net/http"
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
	flag.Parse()
	var (
		app         = app2.NewApp()
		ctx, cancel = context.WithCancel(context.Background())
	)
	// FIXME move to other place
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		if err := http.ListenAndServe(":12527", nil); err != nil {
			logger.Logger.Error("listen and serve failed", zap.Error(err))
		}
	}()

	app.Start(ctx)
	fmt.Println("Starting with version: ", app.Version())
	fmt.Println(logo())
	fmt.Println("Wish you enjoy it!")
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-quit
	cancel()
	time.Sleep(1 * time.Second)
	logger.Logger.Info("exist successfully")
	os.Exit(0)
}
func logo() string {
	return `
   _____ _       _______            
  / ____| |     |__   __|           
 | (___ | | ___   _| |_ __ ___  ___ 
  \___ \| |/ / | | | | '__/ _ \/ _ \
  ____) |   <| |_| | | | |  __/  __/
 |_____/|_|\_\\__, |_|_|  \___|\___|
               __/ |                
              |___/                 
`
}
