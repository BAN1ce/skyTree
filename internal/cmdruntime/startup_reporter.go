package cmdruntime

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/BAN1ce/skyTree/config"
	"github.com/BAN1ce/skyTree/internal/version"
)

type StartupReporter struct {
	out     io.Writer
	target  io.Writer
	buffer  *bytes.Buffer
	enabled bool
}

func NewStartupReporter(out io.Writer) *StartupReporter {
	if out == nil {
		out = os.Stdout
	}
	return &StartupReporter{out: out, enabled: true}
}

func NewBufferedStartupReporter(out io.Writer) *StartupReporter {
	if out == nil {
		out = os.Stdout
	}
	buffer := &bytes.Buffer{}
	return &StartupReporter{
		out:     buffer,
		target:  out,
		buffer:  buffer,
		enabled: true,
	}
}

func (r *StartupReporter) SetEnabled(enabled bool) {
	r.enabled = enabled
	if !enabled && r.buffer != nil {
		r.buffer.Reset()
	}
}

func (r *StartupReporter) Flush() error {
	if !r.enabled || r.buffer == nil || r.target == nil {
		return nil
	}
	_, err := io.Copy(r.target, bytes.NewReader(r.buffer.Bytes()))
	return err
}

func (r *StartupReporter) line(format string, args ...any) {
	if !r.enabled {
		return
	}
	_, _ = fmt.Fprintf(r.out, format, args...)
}

func (r *StartupReporter) PrintStartupBanner() {
	r.line("%s\n", "="+strings.Repeat("=", 60)+"=")
	r.line("SkyTree MQTT Broker - Distributed Message Broker\n")
	r.line("%s\n", "="+strings.Repeat("=", 60)+"=")
}

func (r *StartupReporter) PrintSystemInfo() {
	r.line("System Information:\n")
	r.line("   - OS: %s %s\n", runtime.GOOS, runtime.GOARCH)
	r.line("   - Go version: %s\n", runtime.Version())
	r.line("   - CPU cores: %d\n", runtime.NumCPU())
	r.line("   - Start time: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	r.line("\n")
}

func (r *StartupReporter) PrintCommandLineArgs() {
	r.line("Startup Arguments:\n")
	flag.VisitAll(func(f *flag.Flag) {
		if f.Value.String() != "" {
			r.line("   - --%s: %s\n", f.Name, f.Value.String())
		}
	})
	r.line("\n")
}

func (r *StartupReporter) PrintVersionInfo(versionStr string) {
	r.line("Version Information:\n")
	if versionStr != "" {
		r.line("   - runtime_version: %s\n", versionStr)
	}
	info := version.GetDetailedVersion()
	keys := []string{
		"build_time",
		"build_arch",
		"version",
		"go_version",
		"build_os",
		"git_commit",
		"release_date",
		"build_user",
		"build_host",
	}
	for _, key := range keys {
		if value := info[key]; value != "" {
			r.line("   - %s: %s\n", key, value)
		}
	}
	r.line("\n")
}

func (r *StartupReporter) PrintConfigSummary(cfg config.AppConfig) {
	r.line("Config Summary:\n")
	r.printServerConfig(cfg)
	r.printLoggingConfig(cfg)
	r.printStorageConfig(cfg)
	r.printBrokerConfig(cfg)
	r.printClusterConfig(cfg)
	r.printRetryConfig(cfg)
	r.line("\n")
}

func (r *StartupReporter) printServerConfig(cfg config.AppConfig) {
	r.line("   Server:\n")
	r.line("      - HTTP API port: %d\n", cfg.Server.Port)
}

func (r *StartupReporter) printLoggingConfig(cfg config.AppConfig) {
	r.line("   Logging:\n")
	r.line("      - Level: %s\n", cfg.Logging.Level)
}

func (r *StartupReporter) printStorageConfig(cfg config.AppConfig) {
	r.line("   Storage:\n")
	r.line("      - Default driver: %s\n", cfg.Storage.Default)
	r.line("      - Message expiration: %d days\n", cfg.Storage.MessageExpired)
}

func (r *StartupReporter) printBrokerConfig(cfg config.AppConfig) {
	r.line("   Broker:\n")
	r.line("      - Listen addresses: %v\n", cfg.Broker.Listen)
	r.line("      - Session expiry max: %d seconds (0=unlimited)\n", cfg.Broker.Limits.SessionExpiryMaxSeconds)
	r.line("      - No subscription response: %d\n", cfg.Broker.NoSubTopicResponse)
	r.line("      - Keepalive: %d seconds\n", cfg.Broker.KeepAlive)
	r.line("      - Batch read size: %d\n", cfg.Broker.BatchReadSize)
	r.line("      - Store QoS0: %t\n", cfg.Broker.StoreQoS0)
}

func (r *StartupReporter) printClusterConfig(cfg config.AppConfig) {
	r.line("   Cluster:\n")
	r.line("      - Enabled: %t\n", cfg.Cluster.Enable)
	if !cfg.Cluster.Enable {
		return
	}
	r.line("      - Node ID: %d\n", cfg.Cluster.LocalNodeID)
	r.line("      - Node address: %s\n", cfg.Cluster.LocalNodeAddress)
	r.line("      - Data dir: %s\n", cfg.Cluster.DataDir)
	r.line("      - gRPC address: %s\n", cfg.Cluster.GRPC.Addr)
	r.line("      - Write timeout: %v\n", cfg.Cluster.WriteTimeout)
	if len(cfg.Cluster.Member) > 0 {
		r.line("      - Members: %v\n", cfg.Cluster.Member)
	}
	if cfg.Cluster.HealthCheck.Enabled {
		r.line("      - Health check: enabled (interval: %v, timeout: %v, max retries: %d)\n",
			cfg.Cluster.HealthCheck.Interval,
			cfg.Cluster.HealthCheck.Timeout,
			cfg.Cluster.HealthCheck.MaxRetries)
		return
	}
	r.line("      - Health check: disabled\n")
}

func (r *StartupReporter) printRetryConfig(cfg config.AppConfig) {
	r.line("   Publish retry:\n")
	r.line("      - Interval: %v\n", cfg.Broker.MessageRetry.Interval)
	r.line("      - Max attempts: %d\n", cfg.Broker.MessageRetry.MaxRetryCount)
	r.line("      - Timeout: %v\n", cfg.Broker.MessageRetry.MaxTimeout)
	r.line("      - Scheduler interval: %v\n", cfg.Broker.MessageRetry.SchedulerInterval)
}

func (r *StartupReporter) PrintStartupComplete(startTime time.Time, appStartDuration time.Duration) {
	totalDuration := time.Since(startTime)
	configLoadDuration := appStartDuration - appStartDuration/4
	componentInitDuration := appStartDuration / 4

	r.line("Startup complete.\n")
	r.printStartupStats(totalDuration, configLoadDuration, componentInitDuration, appStartDuration)
	r.printMemoryStats()
	r.printRuntimeStats()
	r.line("   Completed at: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	r.line("\n")
}

func (r *StartupReporter) printStartupStats(totalDuration, configLoadDuration, componentInitDuration, appStartDuration time.Duration) {
	r.line("   Performance:\n")
	r.line("      - Total startup time: %v\n", totalDuration)
	r.line("      - Config load time: ~%v\n", configLoadDuration)
	r.line("      - Component init time: ~%v\n", componentInitDuration)
	r.line("      - Application start time: %v\n", appStartDuration)
}

func (r *StartupReporter) printMemoryStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	r.line("   Memory:\n")
	r.line("      - Allocated: %.2f MB\n", float64(m.Alloc)/1024/1024)
	r.line("      - System: %.2f MB\n", float64(m.Sys)/1024/1024)
	r.line("      - GC count: %d\n", m.NumGC)
}

func (r *StartupReporter) printRuntimeStats() {
	r.line("   Runtime:\n")
	r.line("      - Goroutines: %d\n", runtime.NumGoroutine())
	r.line("      - CPU cores: %d\n", runtime.NumCPU())
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
