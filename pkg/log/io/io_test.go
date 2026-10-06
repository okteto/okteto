// Copyright 2023 The Okteto Authors
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

package io

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIOControllerInitialisation(t *testing.T) {
	l := NewIOController()
	require.NotNil(t, l)
	require.Equal(t, os.Stdout, l.out.out)
	require.Equal(t, os.Stdin, l.in.in)
}

func TestGetters(t *testing.T) {
	l := NewIOController()
	require.NotNil(t, l.In())
	require.IsType(t, &InputController{}, l.In())

	require.NotNil(t, l.Out())
	require.IsType(t, &OutputController{}, l.Out())

	require.NotNil(t, l.Logger())
	require.IsType(t, &oktetoLogger{}, l.Logger())
}

func TestSetLevel(t *testing.T) {
	l := NewIOController()
	assert.Equal(t, DefaultLogLevel, slog.LevelWarn)

	l.SetLevel("debug")
	require.Equal(t, slog.LevelDebug, l.oktetoLogger.slogLeveler.Level())
	require.Equal(t, levelMap[slog.LevelDebug], l.oktetoLogger.logrusLogger.Level)
}

func TestSetOutputFormat(t *testing.T) {
	l := NewIOController()
	assert.IsType(t, &logrus.TextFormatter{}, l.oktetoLogger.logrusFormatter)
	assert.IsType(t, &ttyFormatter{}, l.out.formatter)

	l.SetOutputFormat("plain")
	assert.IsType(t, &logrus.TextFormatter{}, l.oktetoLogger.logrusFormatter)
	assert.IsType(t, &plainFormatter{}, l.out.formatter)

	l.SetOutputFormat("json")
	assert.IsType(t, &logrusJSONFormatter{}, l.oktetoLogger.logrusFormatter)
	assert.IsType(t, &jsonFormatter{}, l.out.formatter)
}

func TestStage(t *testing.T) {
	l := NewIOController()

	l.SetStage("test")
	l.SetOutputFormat("json")
	l.SetStage("test")
	assert.Equal(t, "test", l.oktetoLogger.logrusFormatter.(*logrusJSONFormatter).stage)
}

func TestWithQuietOutputSharesLogger(t *testing.T) {
	ioCtrl := NewIOController()

	quiet := ioCtrl.WithQuietOutput()

	require.Same(t, ioCtrl.Logger(), quiet.Logger())
	require.Same(t, ioCtrl.In(), quiet.In())
}

func TestWithQuietOutputDiscardsOutput(t *testing.T) {
	quiet := NewIOController().WithQuietOutput()

	require.Equal(t, io.Discard, quiet.Out().out)
}

func TestWithQuietOutputNeverDrawsSpinner(t *testing.T) {
	quiet := NewIOController().WithQuietOutput()

	_, isNoSpinner := quiet.Out().Spinner("Waiting for the Okteto Build service").(*noSpinner)

	require.True(t, isNoSpinner)
}

func TestWithQuietOutputIsSafeForConcurrentSpinners(t *testing.T) {
	quiet := NewIOController().WithQuietOutput()

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sp := quiet.Out().Spinner("Waiting for BuildKit service to be ready...")
			sp.Start()
			quiet.Out().Println("message")
			sp.Stop()
		}()
	}
	wg.Wait()

	require.Nil(t, quiet.Out().spinner)
}

func TestWithOutputWritesToGivenWriter(t *testing.T) {
	ioCtrl := NewIOController()
	ioCtrl.SetOutputFormat("plain")
	var out bytes.Buffer

	child := ioCtrl.WithOutput(&out, "Building service api")
	child.Out().Println("hello")

	require.Equal(t, "hello\n", out.String())
}

func TestWithOutputTagsStage(t *testing.T) {
	ioCtrl := NewIOController()
	ioCtrl.SetOutputFormat("json")
	var out bytes.Buffer

	child := ioCtrl.WithOutput(&out, "Building service api")
	child.Out().Println("hello")

	require.Contains(t, out.String(), `"stage":"Building service api"`)
}

func TestWithOutputLeavesParentStage(t *testing.T) {
	ioCtrl := NewIOController()
	ioCtrl.SetOutputFormat("json")
	ioCtrl.SetStage("Building services")
	t.Cleanup(func() { ioCtrl.SetStage("") })
	var parentOut, childOut bytes.Buffer
	ioCtrl.out.out = &parentOut

	ioCtrl.WithOutput(&childOut, "Building service api")
	ioCtrl.Out().Println("hello")

	require.Contains(t, parentOut.String(), `"stage":"Building services"`)
}

func TestWithOutputKeepsParentFormat(t *testing.T) {
	ioCtrl := NewIOController()
	ioCtrl.SetOutputFormat("plain")

	child := ioCtrl.WithOutput(&bytes.Buffer{}, "Building service api")

	_, isPlain := child.Out().formatter.(*plainFormatter)
	require.True(t, isPlain)
}

func TestWithOutputSharesLoggerAndNeverDrawsSpinner(t *testing.T) {
	ioCtrl := NewIOController()

	child := ioCtrl.WithOutput(&bytes.Buffer{}, "Building service api")
	_, isNoSpinner := child.Out().Spinner("waiting").(*noSpinner)

	require.Same(t, ioCtrl.Logger(), child.Logger())
	require.True(t, isNoSpinner)
}
