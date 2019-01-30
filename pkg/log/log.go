package log

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/cloudnativedevelopment/cnd/pkg/config"

	"github.com/Sirupsen/logrus"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

type logger struct {
	out  *logrus.Logger
	file *logrus.Logger
}

var log = &logger{}

// Init configures the logger for the package to use.
func Init(level logrus.Level) {
	log = &logger{}
	log.out = logrus.New()
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
		MaxSize:    10, // megabytes
		MaxBackups: 3,
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

func Debug(args ...interface{}) {
	log.out.Debug(args...)
	log.file.Debug(args...)
}

func Debugf(format string, args ...interface{}) {
	log.out.Debugf(format, args...)
	log.file.Debugf(format, args...)
}

func Info(args ...interface{}) {
	log.out.Info(args...)
	log.file.Info(args...)
}

func Infof(format string, args ...interface{}) {
	log.out.Infof(format, args...)
	log.file.Infof(format, args...)

}

func Error(args ...interface{}) {
	log.out.Error(args...)
	log.file.Error(args...)
}

func Errorf(format string, args ...interface{}) {
	log.out.Errorf(format, args...)
	log.file.Errorf(format, args...)
}
