package log

import (
	"os"

	"github.com/Sirupsen/logrus"
)

// Log configured for the package to consume
var Log *logrus.Logger

// Init configures the logger for the package to use
func Init(level logrus.Level) {
	Log = logrus.New()
	Log.SetOutput(os.Stdout)
	Log.SetLevel(level)
}

// SetLevel sets the level of the main logger
func SetLevel(level string) {
	l, err := logrus.ParseLevel(level)
	if err == nil {
		Log.SetLevel(l)
	}
}
