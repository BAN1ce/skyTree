package version

import "runtime"

const (
	Version = "v0.1.0"
)

var (
	GitCommit   string
	ReleaseDate string
	GoVersion   string
)

func GetVersion() string {
	GoVersion = runtime.Version()
	return Version + "-" + GitCommit + "-" + ReleaseDate + "-" + GoVersion
}
