// Copyright 2026 The Okteto Authors
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
	"strings"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// logConcurrently runs every function n times, all of them at the same time
func logConcurrently(n int, fns ...func(i int)) {
	var wg sync.WaitGroup
	for _, fn := range fns {
		wg.Add(1)
		go func(fn func(i int)) {
			defer wg.Done()
			for i := 0; i < n; i++ {
				fn(i)
			}
		}(fn)
	}
	wg.Wait()
}

func countLinesWithPrefix(output, prefix string) int {
	count := 0
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, prefix) {
			count++
		}
	}
	return count
}

func TestLoggerIsSafeForConcurrentUse(t *testing.T) {
	Init(logrus.WarnLevel)
	// lines are only added to the buffer when there is a stage
	SetStage("deploy")
	t.Cleanup(func() {
		SetStage("")
		Init(logrus.WarnLevel)
	})

	logConcurrently(100,
		func(i int) { Println(fmt.Sprintf("dependency line %d", i)) },
		func(i int) { Println(fmt.Sprintf("main line %d", i)) },
		func(i int) { SetStage(fmt.Sprintf("stage %d", i)) },
		func(i int) { AddMaskedWord(fmt.Sprintf("secret-%d", i)) },
		func(int) { EnableMasking(); DisableMasking() },
		func(i int) { AddToBuffer(InfoLevel, "buffered line %d", i) },
	)

	output := GetOutputBuffer().String()
	require.Equal(t, 100, countLinesWithPrefix(output, "dependency line"))
	require.Equal(t, 100, countLinesWithPrefix(output, "main line"))
}

// enableTestSpinner makes the spinner run, writing to io.Discard instead of the terminal
func enableTestSpinner(t *testing.T) {
	t.Helper()
	Init(logrus.WarnLevel)
	log.spinner.spinnerSupport = true
	log.spinner.sp.Writer = io.Discard
	t.Cleanup(func() {
		StopSpinner()
		Init(logrus.WarnLevel)
	})
}

func TestSpinnerIsSafeForConcurrentUse(t *testing.T) {
	enableTestSpinner(t)

	logConcurrently(100,
		func(i int) { Spinner(fmt.Sprintf("building image %d", i)); StartSpinner() },
		func(int) { StopSpinner() },
		func(i int) { Println(fmt.Sprintf("dependency line %d", i)) },
		func(i int) { Println(fmt.Sprintf("other dependency line %d", i)) },
	)
}

// runningTestSpinner makes the spinner look active: it never runs outside a terminal
func runningTestSpinner(t *testing.T) {
	t.Helper()
	enableTestSpinner(t)
	log.spinner.isActive = func() bool { return true }
}

func TestSpinnerStoppedWhilePausedIsNotResumed(t *testing.T) {
	runningTestSpinner(t)

	// a dependency pauses the spinner to print a line, and the main deploy stops it meanwhile
	log.spinner.hold()
	StopSpinner()
	// the explicit stop cancels the pending resume
	require.Zero(t, log.spinner.holds)

	log.spinner.unhold()
	require.Zero(t, log.spinner.holds)
}

func TestSpinnerIsResumedAfterTheLastConcurrentPrint(t *testing.T) {
	runningTestSpinner(t)

	// two dependencies print at the same time: the spinner resumes when both have finished
	log.spinner.hold()
	log.spinner.hold()
	log.spinner.unhold()
	require.Equal(t, 1, log.spinner.holds)

	log.spinner.unhold()
	require.Zero(t, log.spinner.holds)
}

func TestOutputBufferCanBeReadWhileLogging(t *testing.T) {
	Init(logrus.WarnLevel)
	SetStage("deploy")
	t.Cleanup(func() {
		SetStage("")
		Init(logrus.WarnLevel)
	})

	logConcurrently(100,
		func(i int) { Println(fmt.Sprintf("dependency line %d", i)) },
		func(int) { _ = GetOutputBuffer().String() },
	)

	require.Equal(t, 100, countLinesWithPrefix(GetOutputBuffer().String(), "dependency line"))
}

// readers of the deploy output can't modify it
func TestOutputBufferIsACopy(t *testing.T) {
	Init(logrus.WarnLevel)
	SetStage("deploy")
	t.Cleanup(func() {
		SetStage("")
		Init(logrus.WarnLevel)
	})
	Println("line")

	GetOutputBuffer().Reset()

	require.Equal(t, 1, countLinesWithPrefix(GetOutputBuffer().String(), "line"))
}
