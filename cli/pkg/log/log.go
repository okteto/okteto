package log

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
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

	// InformationSymbol is a checkmark with the information color applied
	InformationSymbol = BlueString(" ⓘ ")
)

type logger struct {
	out *logrus.Logger
}

var log = &logger{
	out: logrus.New(),
}

// Init configures the logger for the package to use.
func Init(level logrus.Level) {
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

// Fatal writes a fatal-level log
func Fatal(args ...interface{}) {
	log.out.Fatal(args...)
}

// Error writes a error-level log
func Error(args ...interface{}) {
	log.out.Error(args...)
}

// Errorf writes a error-level log with a format
func Errorf(format string, args ...interface{}) {
	log.out.Errorf(format, args...)
}

// Red writes a line in red
func Red(format string, args ...interface{}) {
	fmt.Println(RedString(format, args...))
}

// Yellow writes a line in yellow
func Yellow(format string, args ...interface{}) {
	fmt.Println(YellowString(format, args...))
}

// Green writes a line in green
func Green(format string, args ...interface{}) {
	fmt.Println(GreenString(format, args...))
}
