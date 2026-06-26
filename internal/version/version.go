package version

import (
	"fmt"
	"runtime"
	"time"
)

const (
	Version = "v0.1.0-alpha"
)

var (
	GitCommit   string
	ReleaseDate string
	GoVersion   string
	BuildTime   string
	BuildUser   string
	BuildHost   string
	BuildOS     string
	BuildArch   string
)

func init() {
	if GoVersion == "" {
		GoVersion = runtime.Version()
	}
	if BuildOS == "" {
		BuildOS = runtime.GOOS
	}
	if BuildArch == "" {
		BuildArch = runtime.GOARCH
	}
	if BuildTime == "" {
		BuildTime = time.Now().Format("2006-01-02 15:04:05")
	}
}

func GetVersion() string {
	return Version + "-" + GitCommit + "-" + ReleaseDate + "-" + GoVersion
}

// GetDetailedVersion 获取详细的版本信息
func GetDetailedVersion() map[string]string {
	return map[string]string{
		"version":      Version,
		"git_commit":   GitCommit,
		"release_date": ReleaseDate,
		"go_version":   GoVersion,
		"build_time":   BuildTime,
		"build_user":   BuildUser,
		"build_host":   BuildHost,
		"build_os":     BuildOS,
		"build_arch":   BuildArch,
	}
}

// PrintVersionInfo 打印版本信息
func PrintVersionInfo() {
	fmt.Println("📦 详细版本信息:")
	info := GetDetailedVersion()
	for key, value := range info {
		if value != "" {
			fmt.Printf("   • %s: %s\n", key, value)
		}
	}
}
