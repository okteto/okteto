// Copyright 2022 The Okteto Authors
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

package displayer

import (
	"bufio"
	"context"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/manifoldco/promptui/screenbuf"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"golang.org/x/term"
)

type ttyDisplayer struct {
	stdoutScanner *bufio.Scanner
	stderrScanner *bufio.Scanner
	screenbuf     *screenbuf.ScreenBuf

	commandContext context.Context
	cancel         context.CancelFunc

	linesToDisplay        []string
	isBuilding            bool
	buildingpreviousLines int
}

func newTTYDisplayer(stdout, stderr io.Reader) *ttyDisplayer {
	var (
		stdoutScanner *bufio.Scanner
		stderrScanner *bufio.Scanner
	)
	if stdout != nil {
		stdoutScanner = bufio.NewScanner(stdout)
	}
	if stderr != nil {
		stderrScanner = bufio.NewScanner(stderr)
	}

	return &ttyDisplayer{
		stdoutScanner: stdoutScanner,
		stderrScanner: stderrScanner,
		screenbuf:     screenbuf.New(os.Stdout),

		linesToDisplay: []string{},
	}
}

func (d *ttyDisplayer) Display(command string) {
	d.commandContext, d.cancel = context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wgDelta := 0
	if d.stdoutScanner != nil {
		wgDelta++
	}
	if d.stderrScanner != nil {
		wgDelta++
	}
	wg.Add(wgDelta)
	if d.stdoutScanner != nil {
		go func() {
			for d.stdoutScanner.Scan() {
				select {
				case <-d.commandContext.Done():
				default:
					line := d.stdoutScanner.Text()
					if isTopDisplay(line) {
						prevState := d.isBuilding
						d.isBuilding = checkIfIsBuildingLine(line)
						if d.isBuilding && d.isBuilding != prevState {
							d.buildingpreviousLines = len(d.linesToDisplay)
						}
						sanitizedLine := strings.ReplaceAll(line, cursorUp, "")
						sanitizedLine = strings.ReplaceAll(sanitizedLine, resetLine, "")
						d.linesToDisplay = append(d.linesToDisplay[:d.buildingpreviousLines-1], sanitizedLine)
					} else {
						d.linesToDisplay = append(d.linesToDisplay, line)
					}
					if os.Stdout == oktetoLog.GetOutput() {
						oktetoLog.AddToBuffer(oktetoLog.InfoLevel, line)
					}
					width, _, _ := term.GetSize(int(os.Stdout.Fd()))
					lines := renderLines(d.linesToDisplay, width)
					for _, line := range lines {
						d.screenbuf.Write(line)
					}
					d.screenbuf.Flush()
					continue
				}
				break
			}
			if d.stdoutScanner.Err() != nil {
				oktetoLog.Infof("Error reading command output: %s", d.stdoutScanner.Err().Error())
			}
			wg.Done()
		}()
	}

	if d.stderrScanner != nil {
		go func() {
			for d.stderrScanner.Scan() {
				select {
				case <-d.commandContext.Done():
				default:
					line := d.stderrScanner.Text()
					oktetoLog.FWarning(os.Stdout, line)
					continue
				}
				break
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

// CleanUp stops displaying
func (d *ttyDisplayer) CleanUp(_ error) {
	d.cancel()
	<-d.commandContext.Done()
}
