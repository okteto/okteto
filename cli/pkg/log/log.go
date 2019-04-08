package log

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"cli/cnd/pkg/config"
	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
	"k8s.io/klog"
)

var (
	// RedString adds the red ansi color and applies the format
	RedString = color.New(color.FgHiRed).SprintfFunc()

	// GreenString adds the green ansi color and applies the format
	GreenString = color.New(color.FgHiGreen).SprintfFunc()

	// YellowString adds the yellow ansi color and applies the format
	YellowString = color.New(color.FgHiYellow).SprintfFunc()

	// BlueString adds the blue ansi color and applies the format
	BlueString = color.New(color.FgHiBlue).SprintfFunc()

	// SymbolString adds the terminal's background color as foreground, and green as background and applies the format
	SymbolString = color.New(color.BgGreen, color.FgBlack).SprintfFunc()

	// ErrorSymbolString adds the terminal's background color as foreground, and red as background and applies the format
	ErrorSymbolString = color.New(color.BgHiRed, color.FgBlack).SprintfFunc()

	// ErrorSymbol is an X with the error color applied
	ErrorSymbol = ErrorSymbolString(" ✕ ")

	// SuccessSymbol is a checkmark with the success color applied
	SuccessSymbol = SymbolString(" ✓ ")
)

type logger struct {
	out  *logrus.Logger
	file *logrus.Entry
}

var log = &logger{
	out: logrus.New(),
}

// Init configures the logger for the package to use.
func Init(level logrus.Level, actionID string) {
	log.out.SetOutput(os.Stdout)
	log.out.SetLevel(level)

	fileLogger := logrus.New()
	fileLogger.SetFormatter(&logrus.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})

	logPath := filepath.Join(config.GetCNDHome(), fmt.Sprintf("%s%s", config.GetBinaryName(), ".log"))
	rolling := getRollingLog(logPath)
	fileLogger.SetOutput(rolling)
	fileLogger.SetLevel(logrus.DebugLevel)
	log.file = fileLogger.WithFields(logrus.Fields{"action": actionID})

	klog.InitFlags(nil)
	flag.Set("logtostderr", "false")
	flag.Set("alsologtostderr", "false")
	flag.Parse()
	klog.SetOutput(log.file.Writer())
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

// Red writes a line in red
func Red(format string, args ...interface{}) {
	fmt.Println(RedString(format, args...))
	log.file.Errorf(format, args...)
}

// Yellow writes a line in yellow
func Yellow(format string, args ...interface{}) {
	fmt.Println(YellowString(format, args...))
	log.file.Warnf(format, args...)
}

// Green writes a line in green
func Green(format string, args ...interface{}) {
	fmt.Println(GreenString(format, args...))
	log.file.Infof(format, args...)
}
