package logger

import (
	"github.com/sirupsen/logrus"
	"os"
	"time"
)

var Logger = &logrus.Logger{
	Out:   os.Stdout,
	Hooks: nil,
	Formatter: &logrus.TextFormatter{
		TimestampFormat: time.DateTime + ".000",
		FullTimestamp:   true,
	},
	Level:      logrus.DebugLevel,
	BufferPool: nil,
}

func init() {
	Logger.SetLevel(logrus.DebugLevel)
}
