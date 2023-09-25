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
	"os"
	"syscall"
)

type oktetoProcessInterface interface {
	Getpid() int
	Getpgid(pid int) (int, error)
	Find() error
	Signal(os.Signal) error
	Wait() (*os.ProcessState, error)
	Kill() error
	IsProcessSessionLeader() (bool, error)
}

type OktetoProcess struct {
	pid     int
	process *os.Process
}

func newOktetoProcess(pid int) oktetoProcessInterface {
	return &OktetoProcess{
		pid: pid,
	}
}

func (p *OktetoProcess) Getpid() int {
	return p.pid
}

func (p *OktetoProcess) Getpgid(pid int) (int, error) {
	return syscall.Getpgid(pid)
}

func (p *OktetoProcess) Find() error {
	osProcess, err := os.FindProcess(p.pid)
	if err != nil {
		return err
	}
	p.process = osProcess
	return nil
}

func (p *OktetoProcess) Kill() error {
	return p.process.Kill()
}

func (p *OktetoProcess) Signal(signal os.Signal) error {
	return p.process.Signal(signal)
}

func (p *OktetoProcess) Wait() (*os.ProcessState, error) {
	state, err := p.process.Wait()
	return state, err
}
