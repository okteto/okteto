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

package process

import (
	"os"
)

type Interface interface {
	Getpid() int
	Find() error
	Signal(os.Signal) error
	Wait() (*os.ProcessState, error)
	Kill() error
}

type Process struct {
	process *os.Process
	pid     int
}

func New(pid int) Interface {
	return &Process{
		pid: pid,
	}
}

func (p *Process) Getpid() int {
	return p.pid
}

func (p *Process) Find() error {
	osProcess, err := os.FindProcess(p.pid)
	if err != nil {
		return err
	}
	p.process = osProcess
	return nil
}

func (p *Process) Signal(signal os.Signal) error {
	return p.process.Signal(signal)
}

func (p *Process) Wait() (*os.ProcessState, error) {
	return p.process.Wait()
}
