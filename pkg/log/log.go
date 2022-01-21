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

	errorSymbol        = " x "
	coloredErrorSymbol = color.New(color.BgHiRed, color.FgBlack).Sprint(errorSymbol)

	successSymbol        = " ✓ "
	coloredSuccessSymbol = color.New(color.BgGreen, color.FgBlack).Sprint(successSymbol)

	informationSymbol        = " i "
	coloredInformationSymbol = color.New(color.BgHiBlue, color.FgBlack).Sprint(informationSymbol)

	warningSymbol        = " ! "
	coloredWarningSymbol = color.New(color.BgHiYellow, color.FgBlack).Sprint(warningSymbol)

	questionSymbol        = " ? "
	coloredQuestionSymbol = color.New(color.BgHiMagenta, color.FgBlack).Sprint(questionSymbol)
)

type logger struct {
	out        *logrus.Logger
	file       *logrus.Entry
	writer     OktetoWriter
	stage      string
	outputMode string
}

var log = &logger{
	out: logrus.New(),
}

func init() {
	if runtime.GOOS == "windows" {
		successSymbol = color.New(color.BgGreen, color.FgBlack).Sprint(" + ")
	}
	Init(logrus.WarnLevel)
}

// Init configures the logger for the package to use.
func Init(level logrus.Level) {
	log.out.SetOutput(os.Stdout)
	log.out.SetLevel(level)
	log.writer = log.getWriter(TTYFormat)
}

//ConfigureFileLogger configures the file to write
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

//GetOutputFormat returns the output format of the command
func GetOutputFormat() string {
	return log.outputMode
}

// SetOutputFormat sets the output format
func SetOutputFormat(format string) {
	log.writer = log.getWriter(format)
}

// SetStage sets the stage of the logger
func SetStage(stage string) {
	log.stage = stage
}

// IsDebug checks if the level of the main logger is DEBUG or TRACE
func IsDebug() bool {
	return log.out.GetLevel() >= logrus.DebugLevel
}

// Debug writes a debug-level log
func Debug(args ...interface{}) {
	log.writer.Debug(args...)
}

// Debugf writes a debug-level log with a format
func Debugf(format string, args ...interface{}) {
	log.writer.Debugf(format, args...)
}

// Info writes a info-level log
func Info(args ...interface{}) {
	log.writer.Info(args...)
}

// Infof writes a info-level log with a format
func Infof(format string, args ...interface{}) {
	log.writer.Infof(format, args...)
}

// Error writes a error-level log
func Error(args ...interface{}) {
	log.writer.Error(args...)
}

// Errorf writes a error-level log with a format
func Errorf(format string, args ...interface{}) {
	log.writer.Errorf(format, args...)
}

// Fatalf writes a error-level log with a format
func Fatalf(format string, args ...interface{}) {
	log.writer.Fatalf(format, args...)
}

// Yellow writes a line in yellow
func Yellow(format string, args ...interface{}) {
	log.writer.Yellow(format, args...)
}

// Green writes a line in green
func Green(format string, args ...interface{}) {
	log.writer.Green(format, args...)
}

// BlueString returns a string in blue
func BlueString(format string, args ...interface{}) string {
	return blueString(format, args...)
}

// BlueBackgroundString returns a string in a blue background
func BlueBackgroundString(format string, args ...interface{}) string {
	return blueString(format, args...)
}

// Success prints a message with the success symbol first, and the text in green
func Success(format string, args ...interface{}) {
	log.writer.Success(format, args...)
}

// Information prints a message with the information symbol first, and the text in blue
func Information(format string, args ...interface{}) {
	log.writer.Information(format, args...)
}

// Question prints a message with the question symbol first, and the text in magenta
func Question(format string, args ...interface{}) error {
	return log.writer.Question(format, args...)
}

// Warning prints a message with the warning symbol first, and the text in yellow
func Warning(format string, args ...interface{}) {
	log.writer.Warning(format, args...)
}

// Hint prints a message with the text in blue
func Hint(format string, args ...interface{}) {
	log.writer.Hint(format, args...)
}

// Fail prints a message with the error symbol first, and the text in red
func Fail(format string, args ...interface{}) {
	log.writer.Fail(format, args...)
}

// Println writes a line with colors
func Println(args ...interface{}) {
	log.writer.Println(args...)
}

// Print writes a line with colors
func Print(args ...interface{}) {
	log.writer.Print(args...)
}

// Printf writes a line with format
func Printf(format string, args ...interface{}) {
	log.writer.Printf(format, args...)
}

//IsInteractive checks if the writer is interactive
func IsInteractive() bool {
	return log.writer.IsInteractive()
}
