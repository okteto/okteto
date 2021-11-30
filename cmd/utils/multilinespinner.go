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

package utils

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/manifoldco/promptui/screenbuf"
)

//MultilineSpinner represents an okteto multiline spinner
type MultilineSpinner struct {
	suffix             string
	spinnerStyle       []string
	mu                 *sync.RWMutex
	active             bool
	delay              time.Duration
	HideCursor         bool
	stopChan           chan struct{}
	sb                 *screenbuf.ScreenBuf
	multilines         map[string]string
	multilinesFinished map[string]bool
}

//NewMultilineSpinner returns a new Spinner
func NewMultilineSpinner(ctx context.Context, suffix string) *MultilineSpinner {
	spinnerSupport = !loadBoolean("OKTETO_DISABLE_SPINNER")

	return &MultilineSpinner{
		suffix:             suffix,
		spinnerStyle:       []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		delay:              100 * time.Millisecond,
		mu:                 &sync.RWMutex{},
		HideCursor:         true,
		stopChan:           make(chan struct{}),
		multilines:         map[string]string{},
		multilinesFinished: map[string]bool{},
	}
}

// Start will start the indicator.
func (s *MultilineSpinner) Start() {
	s.mu.Lock()
	if s.active {
		s.mu.Unlock()
		return
	}
	if s.HideCursor && runtime.GOOS != "windows" {
		fmt.Print("\033[?25l")
	}
	s.active = true
	s.mu.Unlock()

	s.sb = screenbuf.New(os.Stdout)
	go func() {
		for {
			for i := 0; i < len(s.spinnerStyle); i++ {
				select {
				case <-s.stopChan:
					return
				default:
					if !s.active {
						return
					}
					s.mu.Lock()
					s.sb.Reset()
					whitespaces := ""
					if s.suffix != "" {
						s.sb.WriteString(fmt.Sprintf("%s %s ", s.spinnerStyle[i], s.suffix))
						whitespaces = "  "
					}
					values := make([]string, 0)
					for k, v := range s.multilines {
						if s.multilinesFinished[k] {
							values = append(values, fmt.Sprintf("%s %s %s ", whitespaces, promptui.IconGood, v))
						} else {
							values = append(values, fmt.Sprintf("%s %s %s ", whitespaces, s.spinnerStyle[i], v))
						}
					}
					sort.Strings(values)
					for _, v := range values {
						s.sb.WriteString(v)
					}
					s.sb.Flush()
					s.mu.Unlock()
					time.Sleep(s.delay)
				}
			}
		}
	}()
}

// Stop stops the indicator.
func (s *MultilineSpinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active {
		s.active = false
		if s.HideCursor && runtime.GOOS != "windows" {
			// makes the cursor visible
			fmt.Print("\033[?25h")
		}
		s.sb.Reset()
		s.sb.Clear()
		s.sb.Flush()
		s.stopChan <- struct{}{}
	}
}

// Restart will stop and start the indicator.
func (s *MultilineSpinner) Restart() {
	s.Stop()
	s.Start()
}

// UpdateLine adds a new line to the spinner or updates a existing one.
func (s *MultilineSpinner) UpdateLine(k, v string) {
	if _, ok := s.multilines[k]; !ok {
		s.multilinesFinished[k] = false
	}
	s.multilines[k] = v
}

// FinishLine will stop the spinner for one line.
func (s *MultilineSpinner) FinishLine(k string) {
	if _, ok := s.multilinesFinished[k]; ok {
		s.multilinesFinished[k] = true
	}
}
