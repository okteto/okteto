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
	"os"
	"time"
	"unicode"

	sp "github.com/briandowns/spinner"
	"golang.org/x/term"
)

const (
	// OktetoDisableSpinnerEnvVar if true spinner is disabled
	OktetoDisableSpinnerEnvVar = "OKTETO_DISABLE_SPINNER"

	// spinnerAndWhitespaceCharCount is the number of characters that the spinner and the whitespace take
	spinnerAndWhitespaceCharCount = 2

	// minimumWidthToTrimSpinnerMsg is the minimum width to trim the spinner message
	minimumWidthToTrimSpinnerMsg = 4

	// charsToRemoveForThreeDots is the maximum number of characters to remove to add the three dots
	charsToRemoveForThreeDots = 5
)

// OktetoSpinner is the interface for the spinner
type OktetoSpinner interface {
	Start()
	Stop()

	getMessage() string
	isActive() bool
}

// ttySpinner is the spinner for the tty
type ttySpinner struct {
	*sp.Spinner

	getTerminalWidth func() (int, error)
	message          string
}

// newTTYSpinner creates a new ttySpinner
func newTTYSpinner(message string) *ttySpinner {
	spinnerRotationSpeed := 100 * time.Millisecond
	spinner := sp.New(sp.CharSets[14], spinnerRotationSpeed, sp.WithHiddenCursor(true))
	okSpinner := &ttySpinner{
		Spinner: spinner,
		message: ucFirst(message),

		getTerminalWidth: getTerminalWidthFunc,
	}
	spinner.PreUpdate = okSpinner.preUpdateFunc()
	return okSpinner
}

// Start starts the spinner
func (s *ttySpinner) Start() {
	s.Spinner.Start()
}

// Stop stops the spinner
func (s *ttySpinner) Stop() {
	s.Spinner.Stop()
}

// getMessage returns the spinner message
func (s *ttySpinner) getMessage() string {
	return s.message
}

// isActive returns true if the spinner is active
func (s *ttySpinner) isActive() bool {
	return s.Spinner.Active()
}

func (s *ttySpinner) preUpdateFunc() func(spinner *sp.Spinner) {
	return func(spinner *sp.Spinner) {
		width, err := s.getTerminalWidth()
		if err != nil {
			return
		}
		spinner.Suffix = " " + s.calculateSuffix(width)
	}
}

func getTerminalWidthFunc() (int, error) {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 0, err
	}
	return width, nil
}

func (s *ttySpinner) calculateSuffix(width int) string {
	if width > minimumWidthToTrimSpinnerMsg &&
		len(s.message)+spinnerAndWhitespaceCharCount > width {
		return s.message[:width-charsToRemoveForThreeDots] + "..."
	}
	return s.message
}

// noSpinner is the spinner for the no tty modes
type noSpinner struct {
	OutputController *OutputController
	msg              string
}

// newNoSpinner creates a new noSpinner
func newNoSpinner(msg string, l *OutputController) *noSpinner {
	return &noSpinner{
		msg:              ucFirst(msg),
		OutputController: l,
	}
}

// Start starts the spinner
func (s *noSpinner) Start() {
	s.OutputController.Println(s.msg)
}

// Stop stops the spinner
func (s *noSpinner) Stop() {}

// getMessage returns the spinner message
func (s *noSpinner) getMessage() string {
	return s.msg
}

// isActive returns true if the spinner is active
func (s *noSpinner) isActive() bool {
	return false
}

// ucFirst returns the string with the first letter in uppercase
func ucFirst(str string) string {
	for i, v := range str {
		return string(unicode.ToUpper(v)) + str[i+1:]
	}
	return ""
}
