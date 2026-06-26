package cmdruntime

import "flag"

type StartupOptions struct {
	ConfigFile string
}

func ParseStartupOptions() StartupOptions {
	configFile := flag.String("config", "./etc/config.yaml", "config file path")
	flag.Parse()
	return StartupOptions{ConfigFile: *configFile}
}
