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

	"github.com/fatih/color"
)

var (
	// coloredSuccessSymbol represents the colored success symbol
	coloredSuccessSymbol = color.New(color.BgGreen, color.FgBlack).Sprint(" ✓ ")

	// coloredInformationSymbol represents the colored information symbol
	coloredInformationSymbol = color.New(color.BgHiBlue, color.FgBlack).Sprint(" i ")

	// coloredWarningSymbol represents the colored warning symbol
	coloredWarningSymbol = color.New(color.BgHiYellow, color.FgBlack).Sprint(" ! ")

	// coloredQuestionSymbol represents the colored question symbol
	coloredQuestionSymbol = color.New(color.BgHiMagenta, color.FgBlack).Sprint(" ? ")

	// greenString is a function that returns a green string
	greenString = color.New(color.FgGreen).SprintfFunc()

	// yellowString is a function that returns a yellow string
	yellowString = color.New(color.FgHiYellow).SprintfFunc()

	// blueString is a function that returns a blue string
	blueString = color.New(color.FgHiBlue).SprintfFunc()
)

// decorator is the interface for the decorator
type decorator interface {
	Success(string) string
	Information(string) string
	Question(string) string
	Warning(string) string
}

// TTYDecorator is the decorator for the TTY
type TTYDecorator struct{}

// newTTYDecorator returns a new TTY decorator
func newTTYDecorator() *TTYDecorator {
	return &TTYDecorator{}
}

// Success decorates a success message
func (d *TTYDecorator) Success(msg string) string {
	return fmt.Sprintf("%s %s\n", coloredSuccessSymbol, greenString(msg))
}

// Information decorates an information message
func (d *TTYDecorator) Information(msg string) string {
	return fmt.Sprintf("%s %s\n", coloredInformationSymbol, blueString(msg))
}

// Question decorates a question message
func (d *TTYDecorator) Question(msg string) string {
	return fmt.Sprintf("%s %s", coloredQuestionSymbol, color.MagentaString(msg))
}

// Warning decorates a warning message
func (d *TTYDecorator) Warning(msg string) string {
	return fmt.Sprintf("%s %s\n", coloredWarningSymbol, yellowString(msg))
}

// PlainDecorator is the decorator for the plain output
type PlainDecorator struct{}

// NewPlainDecorator returns a new plain decorator
func newPlainDecorator() *PlainDecorator {
	return &PlainDecorator{}
}

// Success decorates a success message
func (d *PlainDecorator) Success(msg string) string {
	return fmt.Sprintf("SUCCESS: %s\n", msg)
}

// Information decorates an information message
func (d *PlainDecorator) Information(msg string) string {
	return fmt.Sprintf("INFO: %s\n", msg)
}

// Question decorates a question message
func (d *PlainDecorator) Question(msg string) string {
	return msg
}

// Warning decorates a warning message
func (d *PlainDecorator) Warning(msg string) string {
	return fmt.Sprintf("WARNING: %s\n", msg)
}
