package log

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/cloudnativedevelopment/cnd/pkg/config"

	"github.com/sirupsen/logrus"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

type logger struct {
	out  *logrus.Logger
	file *logrus.Logger
}

var log = &logger{
	out: logrus.New(),
}

// Init configures the logger for the package to use.
func Init(level logrus.Level) {
	log.out.SetOutput(os.Stdout)
	log.out.SetLevel(level)

	log.file = logrus.New()
	log.file.SetFormatter(&logrus.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})

	logPath := path.Join(config.GetCNDHome(), fmt.Sprintf("%s%s", config.GetBinaryName(), ".log"))
	rolling := getRollingLog(logPath)
	log.file.SetOutput(rolling)
	log.file.SetLevel(logrus.DebugLevel)

}

func getRollingLog(path string) io.Writer {
	return &lumberjack.Logger{
		Filename:   path,
		MaxSize:    1, // megabytes
		MaxBackups: 10,
		MaxAge:     28, //days
		Compress:   true,
	}
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
	if log.file != nil {
		log.file.Debug(args...)
	}

}

// Debugf writes a debug-level log with a format
func Debugf(format string, args ...interface{}) {
	log.out.Debugf(format, args...)
	if log.file != nil {
		log.file.Debugf(format, args...)
	}
}

// Info writes a info-level log
func Info(args ...interface{}) {
	log.out.Info(args...)
	if log.file != nil {
		log.file.Info(args...)
	}
}

// Infof writes a info-level log with a format
func Infof(format string, args ...interface{}) {
	log.out.Infof(format, args...)
	if log.file != nil {
		log.file.Infof(format, args...)
	}

}

// Error writes a error-level log
func Error(args ...interface{}) {
	log.out.Error(args...)
	if log.file != nil {
		log.file.Error(args...)
	}
}

// Errorf writes a error-level log with a format
func Errorf(format string, args ...interface{}) {
	log.out.Errorf(format, args...)
	if log.file != nil {
		log.file.Errorf(format, args...)
	}
}
