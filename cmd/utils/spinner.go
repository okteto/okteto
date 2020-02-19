// Copyright 2020 The Okteto Authors
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

package utils

import (
	"fmt"
	"runtime"
	"time"

	sp "github.com/briandowns/spinner"
)

//Spinner represents an okteto spinner
type Spinner struct {
	sp *sp.Spinner
}

//NewSpinner returns a new Spinner
func NewSpinner(suffix string) *Spinner {
	s := sp.New(sp.CharSets[14], 100*time.Millisecond)
	s.HideCursor = true
	s.Suffix = fmt.Sprintf(" %s", suffix)
	return &Spinner{
		sp: s,
	}
}

//Start starts the spinner
func (p *Spinner) Start() {
	if runtime.GOOS == "windows" {
		fmt.Printf(" %s\n", p.sp.Suffix)
		return
	}

	p.sp.Start()
}

//Stop stops the spinner
func (p *Spinner) Stop() {
	if runtime.GOOS == "windows" {
		return
	}

	p.sp.Stop()
}

//Update updates the spinner message
func (p *Spinner) Update(text string) {
	p.sp.Suffix = fmt.Sprintf(" %s", text)
}
