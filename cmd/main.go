package main

import (
	"context"
	"fmt"
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
