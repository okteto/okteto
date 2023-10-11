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
	"fmt"
	"os"
	"time"
	"unicode"

	sp "github.com/briandowns/spinner"
	"golang.org/x/term"
)

const (
	// OktetoDisableSpinnerEnvVar if true spinner is disabled
	OktetoDisableSpinnerEnvVar = "OKTETO_DISABLE_SPINNER"

	// SpinnerAndWhitespaceCharCount is the number of characters that the spinner and the whitespace take
	SpinnerAndWhitespaceCharCount = 2

	// minimumWidthToTrimSpinnerMsg is the minimum width to trim the spinner message
	MinimumWidthToTrimSpinnerMsg = 4

	// maxCharsToRemoveForThreeDots is the maximum number of characters to remove to add the three dots
	CharsToRemoveForThreeDots = 5
)

// Spinner is the interface for the spinner
type OktetoSpinner interface {
	Start()
	Stop()

	getMessage() string
}

// ttySpinner is the spinner for the tty
type ttySpinner struct {
	*sp.Spinner

	message          string
	getTerminalWidth func() (int, error)
}

// newTTYSpinner creates a new ttySpinner
func newTTYSpinner(message string) *ttySpinner {
	spinner := sp.New(sp.CharSets[14], 100*time.Millisecond, sp.WithHiddenCursor(true))
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

func (s *ttySpinner) preUpdateFunc() func(spinner *sp.Spinner) {
	return func(spinner *sp.Spinner) {
		width, err := s.getTerminalWidth()
		if err != nil {
			return
		}
		spinner.Suffix = s.calculateSuffix(width)
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
	if width > MinimumWidthToTrimSpinnerMsg &&
		len(s.message)+SpinnerAndWhitespaceCharCount > width {
		return s.message[:width-CharsToRemoveForThreeDots] + "..."
	}
	return s.message
}

// noSpinner is the spinner for the no tty modes
type noSpinner struct {
	msg string
}

// newNoSpinner creates a new noSpinner
func newNoSpinner(msg string) *noSpinner {
	return &noSpinner{
		msg: ucFirst(msg),
	}
}

// Start starts the spinner
func (s *noSpinner) Start() {
	fmt.Println(s.msg)
}

// Stop stops the spinner
func (s *noSpinner) Stop() {}

// getMessage returns the spinner message
func (s *noSpinner) getMessage() string {
	return s.msg
}

// ucFirst returns the string with the first letter in uppercase
func ucFirst(str string) string {
	for i, v := range str {
		return string(unicode.ToUpper(v)) + str[i+1:]
	}
	return ""
}
