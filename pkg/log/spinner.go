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

package log

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	sp "github.com/briandowns/spinner"
	"golang.org/x/term"
)

const (
	// OktetoDisableSpinnerEnvVar if true spinner is disabled
	OktetoDisableSpinnerEnvVar = "OKTETO_DISABLE_SPINNER"
)

type spinnerLogger struct {
	sp *sp.Spinner
	// isActive reports if the spinner is running. It is sp.Active, replaced in tests because
	// the spinner only runs in a terminal
	isActive       func() bool
	spinnerSupport bool
	// holds counts the prints in progress that paused the running spinner
	holds int
	// mu serializes the spinner operations: dependencies deployed in parallel print lines
	// while the main deploy starts and stops the spinner
	mu sync.Mutex
}

// hold is used within the TTYWriter to pause the spinner to display the log
// if the spinner is Active (running) it will stop
func (sl *spinnerLogger) hold() {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	if sl.holds > 0 {
		sl.holds++
		return
	}
	if !sl.isActive() {
		return
	}
	sl.holds = 1
	sl.stop()
}

// unhold is used within the TTYWriter to restart the spinner after displaying the log.
// The spinner runs again when the last print that paused it has finished, unless it was stopped meanwhile
func (sl *spinnerLogger) unhold() {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	if sl.holds == 0 {
		return
	}
	sl.holds--
	if sl.holds == 0 {
		sl.start()
	}
}

// start runs the spinner. It must be called with sl.mu locked
func (sl *spinnerLogger) start() {
	sl.sp.Lock()
	if sl.sp.FinalMSG == "" {
		sl.sp.FinalMSG = sl.sp.Suffix
	}
	sl.sp.Unlock()
	sl.sp.Start()
}

// stop deletes FinalMSG and stops the spinner. It must be called with sl.mu locked
func (sl *spinnerLogger) stop() {
	sl.sp.Lock()
	sl.sp.FinalMSG = ""
	sl.sp.Unlock()
	if sl.spinnerSupport {
		sl.sp.Stop()
	}
}

func newSpinner() *sp.Spinner {
	rotationSpeed := 100 * time.Millisecond
	spinner := sp.New(sp.CharSets[14], rotationSpeed, sp.WithHiddenCursor(true))
	spinner.PreUpdate = func(spinner *sp.Spinner) {
		width, _, err := term.GetSize(int(os.Stdout.Fd()))
		if err != nil {
			Infof("failed to get terminal size: %s", err)
		}
		if width > 4 && len(spinner.FinalMSG)+2 > width {
			spinner.Suffix = spinner.FinalMSG[:width-5] + "..."
		} else {
			spinner.Suffix = spinner.FinalMSG
		}
	}
	return spinner
}

// Spinner sets the text provided as Suffix and FinalMSG of the spinner instance
func Spinner(text string) {
	log.spinner.mu.Lock()
	defer log.spinner.mu.Unlock()
	log.spinner.sp.Lock()
	defer log.spinner.sp.Unlock()
	log.spinner.sp.Suffix = fmt.Sprintf(" %s", ucFirst(text))
	log.spinner.sp.FinalMSG = log.spinner.sp.Suffix
}

// StartSpinner starts to run the spinner if enabled or Println if not
func StartSpinner() {
	if log.spinner.spinnerSupport {
		log.spinner.mu.Lock()
		defer log.spinner.mu.Unlock()
		log.spinner.start()
		return
	}
	log.spinner.sp.Lock()
	suffix := log.spinner.sp.Suffix
	log.spinner.sp.Unlock()
	Println(strings.TrimSpace(suffix))
}

// StopSpinner deletes FinalMSG and stops the running of the spinner.
// The spinner is not resumed by the prints that paused it before
func StopSpinner() {
	log.spinner.mu.Lock()
	defer log.spinner.mu.Unlock()
	log.spinner.holds = 0
	log.spinner.stop()
}

func ucFirst(str string) string {
	for i, v := range str {
		return string(unicode.ToUpper(v)) + str[i+1:]
	}
	return ""
}
