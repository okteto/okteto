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
	sp             *sp.Spinner
	spinnerSupport bool
	onHold         bool
}

// hold is used within the TTYWritter to pause the spinner to display the log
// if the spinner is Active (running) it will stop
func (sl *spinnerLogger) hold() {
	if !sl.sp.Active() {
		return
	}
	sl.onHold = true
	StopSpinner()
}

// unhold is used within the TTYWritter to restart the spinner after display the log.
// If the spinner is onHold (previously Active) this will start the spinning running again
func (sl *spinnerLogger) unhold() {
	if !sl.onHold {
		return
	}
	sl.onHold = false
	StartSpinner()
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
	log.spinner.sp.Suffix = fmt.Sprintf(" %s", ucFirst(text))
	log.spinner.sp.FinalMSG = log.spinner.sp.Suffix
}

// StartSpinner starts to run the spinner if enabled or Println if not
func StartSpinner() {
	if log.spinner.spinnerSupport {
		if log.spinner.sp.FinalMSG == "" {
			log.spinner.sp.FinalMSG = log.spinner.sp.Suffix
		}
		log.spinner.sp.Start()
	} else {
		Println(strings.TrimSpace(log.spinner.sp.Suffix))
	}
}

// StopSpinner deletes FinalMSG and stops the running of the spinner
func StopSpinner() {
	if log.spinner.sp.FinalMSG != "" {
		log.spinner.sp.FinalMSG = ""
	}
	if log.spinner.spinnerSupport {
		log.spinner.sp.Stop()
	}
}

func ucFirst(str string) string {
	for i, v := range str {
		return string(unicode.ToUpper(v)) + str[i+1:]
	}
	return ""
}
