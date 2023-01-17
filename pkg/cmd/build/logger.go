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

package build

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/moby/buildkit/client"
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

func deployDisplayer(ctx context.Context, ch chan *client.SolveStatus) error {

	// TODO: import build timeout
	timeout := time.NewTicker(10 * time.Minute)
	defer timeout.Stop()

	oktetoLog.Spinner("Preparing remote installer instance...")
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	t := newTrace()

	var done bool
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
		case ss, ok := <-ch:
			if ok {
				if err := t.update(ss); err != nil {
					oktetoLog.Info(err.Error())
					continue
				}
				t.display()
				t.removeCompletedSteps()
			} else {
				done = true
			}
			if done {
				return nil
			}
		}
	}
}

type trace struct {
	ongoing map[string]*vertexInfo
	stages  map[string]bool
}

func newTrace() *trace {
	return &trace{
		ongoing: map[string]*vertexInfo{},
		stages:  map[string]bool{},
	}
}

func (t *trace) update(ss *client.SolveStatus) error {
	for _, rawVertex := range ss.Vertexes {
		v, ok := t.ongoing[rawVertex.Digest.Encoded()]
		if !ok {
			v = &vertexInfo{
				name: rawVertex.Name,
			}
			t.ongoing[rawVertex.Digest.Encoded()] = v
		}
		if rawVertex.Error != "" {
			return fmt.Errorf("error on stage %s: %s", rawVertex.Name, rawVertex.Error)
		}
		if rawVertex.Completed != nil {
			v.completed = true
			continue
		}
	}
	for _, s := range ss.Statuses {
		v, ok := t.ongoing[s.Vertex.Encoded()]
		if !ok {
			continue // shouldn't happen
		}
		if s.Completed != nil {
			v.completed = true
			t.ongoing[s.Vertex.Encoded()] = v
			continue
		}
	}
	for _, l := range ss.Logs {
		v, ok := t.ongoing[l.Vertex.Encoded()]
		if !ok {
			continue // shouldn't happen
		}
		newLogs := strings.Split(string(l.Data), "\n")
		v.logs = append(v.logs, newLogs...)
	}
	return nil
}

func (t trace) display() {
	for _, v := range t.ongoing {
		if t.isTransferringContext(v.name) {
			oktetoLog.Spinner("Synchronising context...")
		}
		if t.hasCommandLogs(v) {
			oktetoLog.StopSpinner()
			for _, log := range v.logs {
				var text oktetoLog.JSONLogFormat
				if err := json.Unmarshal([]byte(log), &text); err != nil {
					oktetoLog.Infof("could not parse %s: %w", log, err)
					continue
				}

				switch text.Stage {
				case "done":
					continue
				case "Load manifest":
					if text.Level == "error" {
						oktetoLog.Fail(text.Message)
					}
				default:
					// Print the information message about the stage if needed
					if _, ok := t.stages[text.Stage]; !ok {
						oktetoLog.Information("Running stage '%s'", text.Stage)
						t.stages[text.Stage] = true
					}
					oktetoLog.Println(text.Message)

				}
			}
			oktetoLog.StartSpinner()
			v.logs = []string{}
		}
	}
}

func (t trace) isTransferringContext(name string) bool {
	isInternal := strings.HasPrefix(name, "[internal]")
	isLoadingCtx := strings.Contains(name, "load build context")
	return isInternal && isLoadingCtx
}

func (t trace) hasCommandLogs(v *vertexInfo) bool {
	if len(v.logs) != 0 {
		return true
	}
	return false
}

func (t *trace) removeCompletedSteps() {
	for k, v := range t.ongoing {
		if v.completed {
			delete(t.ongoing, k)
		}
	}
}

type vertexInfo struct {
	name      string
	completed bool
	logs      []string
}
