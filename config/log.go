package config

type Log struct {
	Level            string `yaml:"level" env:"LOGGING_LEVEL" env-default:"info"`
	File             string `yaml:"file" env:"LOGGING_FILE" env-default:""`
	MaxSize          int    `yaml:"max_size" env:"LOGGING_MAX_SIZE" env-default:"100"`
	MaxAge           int    `yaml:"max_age" env:"LOGGING_MAX_AGE" env-default:"30"`
	MaxBackups       int    `yaml:"max_backups" env:"LOGGING_MAX_BACKUPS" env-default:"10"`
	Compress         bool   `yaml:"compress" env:"LOGGING_COMPRESS" env-default:"true"`
	GinMode          string `yaml:"gin_mode" env:"LOGGING_GIN_MODE" env-default:"release"`
	GinConsoleOutput bool   `yaml:"gin_console_output" env:"LOGGING_GIN_CONSOLE_OUTPUT" env-default:"false"`
	StartupReport    bool   `yaml:"startup_report" env:"LOGGING_STARTUP_REPORT"`
}
