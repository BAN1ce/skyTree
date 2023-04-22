package logger

import "github.com/sirupsen/logrus"

var Logger = logrus.New()

func init() {
	Logger.SetLevel(logrus.DebugLevel)
}
