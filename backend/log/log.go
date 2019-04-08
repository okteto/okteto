package log

import (
	"os"

	"github.com/sirupsen/logrus"
)

type logger struct {
	out *logrus.Logger
}

var log = &logger{
	out: logrus.New(),
}

// Init configures the logger for the package to use.
func Init(level logrus.Level, actionID string) {
	log.out.SetOutput(os.Stdout)
	log.out.SetLevel(level)
}

// SetLevel sets the level of the main logger
func SetLevel(level string) {
	l, err := logrus.ParseLevel(level)
	if err == nil {
		log.out.SetLevel(l)
	}
}

// Debug writes a debug-level log
func Debug(args ...interface{}) {
	log.out.Debug(args...)
}

// Debugf writes a debug-level log with a format
func Debugf(format string, args ...interface{}) {
	log.out.Debugf(format, args...)
}

// Info writes a info-level log
func Info(args ...interface{}) {
	log.out.Info(args...)
}

// Infof writes a info-level log with a format
func Infof(format string, args ...interface{}) {
	log.out.Infof(format, args...)
}

// Error writes a error-level log
func Error(args ...interface{}) {
	log.out.Error(args...)
}

// Errorf writes a error-level log with a format
func Errorf(format string, args ...interface{}) {
	log.out.Errorf(format, args...)
}
