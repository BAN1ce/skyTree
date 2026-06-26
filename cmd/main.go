package main

import (
	"context"
	"os"
	"time"

	"github.com/BAN1ce/skyTree/internal/cmdruntime"
)

// @title           SkyTree API
// @version         1.0

// @host      localhost:8080
// @BasePath  /api/v1

// @securityDefinitions.basic  BasicAuth
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	options := cmdruntime.ParseStartupOptions()
	reporter := cmdruntime.NewBufferedStartupReporter(os.Stdout)
	orchestrator := cmdruntime.NewStartupOrchestrator(reporter)
	result, err := orchestrator.Start(ctx, options)
	if err != nil {
		cmdruntime.ExitWithError("startup failed: %v", err)
	}

	shutdown := cmdruntime.NewShutdownController(3 * time.Second)
	shutdown.Wait(result.App)
	shutdown.Close(result.App, cancel)
	os.Exit(0)
}
