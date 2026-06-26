package cmdruntime

import (
	"context"
	"fmt"
	"time"

	app2 "github.com/BAN1ce/skyTree/internal/app"
	"github.com/BAN1ce/skyTree/config"
)

type StartupResult struct {
	App              *app2.App
	Config           config.AppConfig
	ProcessStartAt   time.Time
	ApplicationStart time.Duration
}

type StartupOrchestrator struct {
	reporter *StartupReporter
}

func NewStartupOrchestrator(reporter *StartupReporter) *StartupOrchestrator {
	if reporter == nil {
		reporter = NewStartupReporter(nil)
	}
	return &StartupOrchestrator{reporter: reporter}
}

func (o *StartupOrchestrator) Start(ctx context.Context, options StartupOptions) (*StartupResult, error) {
	startAt := time.Now()
	o.reporter.PrintStartupBanner()
	o.reporter.PrintSystemInfo()
	o.reporter.PrintCommandLineArgs()
	o.reporter.line("Loading config...\n")
	cfg, err := config.Load(options.ConfigFile)
	if err != nil {
		return nil, fmt.Errorf("load config failed: %w", err)
	}
	o.reporter.SetEnabled(cfg.Logging.StartupReport)
	o.reporter.line("Config loaded successfully (file: %s)\n", options.ConfigFile)

	app, err := app2.NewApp(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("init app failed: %w", err)
	}

	o.reporter.PrintVersionInfo(app.Version())
	o.reporter.PrintConfigSummary(cfg)
	o.reporter.line("Starting SkyTree...\n")
	appStartAt := time.Now()
	if err := app.Start(); err != nil {
		return nil, fmt.Errorf("start app failed: %w", err)
	}
	appStartDuration := time.Since(appStartAt)
	o.reporter.PrintStartupComplete(startAt, appStartDuration)
	o.reporter.line("%s\n", logo())
	o.reporter.line("SkyTree is ready.\n")
	if err := o.reporter.Flush(); err != nil {
		return nil, fmt.Errorf("flush startup report failed: %w", err)
	}

	return &StartupResult{
		App:              app,
		Config:           cfg,
		ProcessStartAt:   startAt,
		ApplicationStart: appStartDuration,
	}, nil
}
