package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/fatih/color"
	"github.com/okteto/okteto/pkg/config"
	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
	lumberjack "gopkg.in/natefinch/lumberjack.v2"
)

var (
	redString = color.New(color.FgHiRed).SprintfFunc()

	greenString = color.New(color.FgGreen).SprintfFunc()

	yellowString = color.New(color.FgHiYellow).SprintfFunc()

	blueString = color.New(color.FgHiBlue).SprintfFunc()

	errorSymbol = color.New(color.BgHiRed, color.FgBlack).Sprint(" x ")

	successSymbol = color.New(color.BgGreen, color.FgBlack).Sprint(" ✓ ")

	informationSymbol = color.New(color.BgHiBlue, color.FgBlack).Sprint(" i ")
)

type logger struct {
	out  *logrus.Logger
	file *logrus.Entry
}

var log = &logger{
	out: logrus.New(),
}

func init() {
	if runtime.GOOS == "windows" {
		successSymbol = color.New(color.BgGreen, color.FgBlack).Sprint(" + ")
	}

}

// Init configures the logger for the package to use.
func Init(level logrus.Level) {

	log.out.SetOutput(os.Stdout)
	log.out.SetLevel(level)

	fileLogger := logrus.New()
	fileLogger.SetFormatter(&logrus.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})

	logPath := filepath.Join(config.GetHome(), "okteto.log")
	rolling := getRollingLog(logPath)
	fileLogger.SetOutput(rolling)
	fileLogger.SetLevel(logrus.DebugLevel)

	actionID := uuid.NewV4().String()
	log.file = fileLogger.WithFields(logrus.Fields{"action": actionID})
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

// Warn writes a warn-level log
func Warn(args ...interface{}) {
	log.out.Warn(args...)
	if log.file != nil {
		log.file.Warn(args...)
	}
}

// Warnf writes a warn	-level log with a format
func Warnf(format string, args ...interface{}) {
	log.out.Warnf(format, args...)
	if log.file != nil {
		log.file.Warnf(format, args...)
	}
}

// Yellow writes a line in yellow
func Yellow(format string, args ...interface{}) {
	log.out.Infof(format, args...)
	fmt.Fprintln(color.Output, yellowString(format, args...))
}

// Green writes a line in green
func Green(format string, args ...interface{}) {
	log.out.Infof(format, args...)
	fmt.Fprintln(color.Output, greenString(format, args...))
}

// BlueString returns a string in blue
func BlueString(format string, args ...interface{}) string {
	return blueString(format, args...)
}

// Success prints a message with the success symbol first, and the text in green
func Success(format string, args ...interface{}) {
	log.out.Infof(format, args...)
	fmt.Fprintf(color.Output, "%s %s\n", successSymbol, greenString(format, args...))
}

// Information prints a message with the information symbol first, and the text in blue
func Information(format string, args ...interface{}) {
	log.out.Infof(format, args...)
	fmt.Fprintf(color.Output, "%s %s\n", informationSymbol, blueString(format, args...))
}

// Hint prints a message with the text in blue
func Hint(format string, args ...interface{}) {
	log.out.Infof(format, args...)
	fmt.Fprintf(color.Output, "%s\n", blueString(format, args...))
}

// Fail prints a message with the error symbol first, and the text in red
func Fail(format string, args ...interface{}) {
	log.out.Infof(format, args...)
	fmt.Fprintf(color.Output, "%s %s\n", errorSymbol, redString(format, args...))
}

// Println writes a line with colors
func Println(args ...interface{}) {
	log.out.Info(args...)
	fmt.Fprintln(color.Output, args...)
}
