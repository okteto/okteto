// Copyright 2024 The Okteto Authors
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

//go:build !windows
// +build !windows

package process

import (
	"errors"
	"syscall"
	"time"

	oktetoLog "github.com/okteto/okteto/pkg/log"
)

// Kill attempts to gracefully shut down the process, wait for the process to exit and if it doesn't, it kills it
func (p *Process) Kill() error {
	err := p.Find()
	if err != nil {
		oktetoLog.Debugf("process not running: %d", p.pid)
		return nil
	}

	// attempt to gracefully terminate the process
	if err := p.Signal(syscall.SIGTERM); err != nil {
		oktetoLog.Debugf("error in graceful termination of process: %d", p.pid)
		return err
	}

	// create a channel to listen for when the process exits
	done := make(chan error, 1)
	go func() {
		_, err := p.Wait()
		done <- err
	}()

	timer := time.NewTimer(3 * time.Second)

	select {
	case <-timer.C:
		oktetoLog.Debugf("graceful termination timed out, killing process: %d", p.pid)
		timer.Stop()
		// if the process is still running, kill it
		if err := p.Signal(syscall.SIGKILL); err != nil {
			oktetoLog.Debugf("error in killing process %d: %v", p.pid, err)
			return err
		}
		err := p.Find()
		if err != nil {
			oktetoLog.Debugf("process %d cannot be : %v", p.pid, err)
			return err
		}
		oktetoLog.Debugf("process terminated successfully: %d", p.pid)
	case err := <-done:
		if errors.Is(err, syscall.ECHILD) {
			timer.Stop()
			oktetoLog.Debugf("process terminated successfully: %d, %v", p.pid, err)
		}
	}

	return nil
}
