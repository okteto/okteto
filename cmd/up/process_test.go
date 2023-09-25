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

package up

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

type mockOktetoProcess struct {
	pid     int
	process *os.Process

	signalsReceived []os.Signal

	findErr                   error
	signalErr                 error
	waitProcessState          *os.ProcessState
	waitErr                   error
	killErr                   error
	isProcessSessionLeader    bool
	isProcessSessionLeaderErr error
}

func (m *mockOktetoProcess) Getpid() int {
	return m.pid
}

func (m *mockOktetoProcess) Find() error {
	if m.findErr != nil {
		return m.findErr
	}

	m.process = &os.Process{Pid: m.pid}

	return nil
}

func (m *mockOktetoProcess) Signal(s os.Signal) error {
	m.signalsReceived = append(m.signalsReceived, s)
	return m.signalErr
}

func (m *mockOktetoProcess) Wait() (*os.ProcessState, error) {
	return m.waitProcessState, m.waitErr
}

func (m *mockOktetoProcess) Kill() error {
	m.signalsReceived = append(m.signalsReceived, os.Kill)
	return m.killErr
}

func (m *mockOktetoProcess) IsProcessSessionLeader() (bool, error) {
	return m.isProcessSessionLeader, m.isProcessSessionLeaderErr
}

func TestOktetoProcess_Getpid(t *testing.T) {
	pid := 1234
	p := newOktetoProcess(pid)

	assert.Equal(t, p.Getpid(), pid)
}
