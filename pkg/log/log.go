// Copyright 2021 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package log

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/fatih/color"
	"github.com/google/uuid"
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

	warningSymbol = color.New(color.BgHiYellow, color.FgBlack).Sprint(" ! ")
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
}

func ConfigureFileLogger(dir, version string) {
	fileLogger := logrus.New()
	fileLogger.SetFormatter(&logrus.TextFormatter{
		DisableColors: true,
		FullTimestamp: true,
	})

	logPath := filepath.Join(dir, "okteto.log")
	rolling := getRollingLog(logPath)
	fileLogger.SetOutput(rolling)
	fileLogger.SetLevel(logrus.DebugLevel)

	actionID := uuid.New().String()
	log.file = fileLogger.WithFields(logrus.Fields{"action": actionID, "version": version})
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

// IsDebug checks if the level of the main logger is DEBUG or TRACE
func IsDebug() bool {
	return log.out.GetLevel() >= logrus.DebugLevel
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

// Fatalf writes a error-level log with a format
func Fatalf(format string, args ...interface{}) {
	if log.file != nil {
		log.file.Errorf(format, args...)
	}

	log.out.Fatalf(format, args...)
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

// Warning prints a message with the warning symbol first, and the text in yellow
func Warning(format string, args ...interface{}) {
	log.out.Infof(format, args...)
	fmt.Fprintf(color.Output, "%s %s\n", warningSymbol, yellowString(format, args...))
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
