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

package syncthing

import (
	"runtime"
	"time"

	"github.com/shirou/gopsutil/process"
)

func terminate(p *process.Process, wait bool) error {

	children, err := getChildren(p)
	if err != nil {
		return err
	}
	err = terminateProccess(p, wait)
	if err != nil {
		return err
	}
	for _, child := range children {
		err := terminateProccess(child, wait)
		if err != nil {
			return err
		}
	}
	return nil
}

func getChildren(p *process.Process) ([]*process.Process, error) {
	if runtime.GOOS == "windows" {
		return p.Children()
	}
	return make([]*process.Process, 0), nil
}

func terminateProccess(p *process.Process, wait bool) error {
	if err := p.Terminate(); err != nil {
		return err
	}

	if !wait {
		return nil
	}

	notRunning, err := waitUntilNotRunning(p)
	if err != nil {
		return err
	}

	if notRunning {
		return nil
	}

	if err := p.Kill(); err != nil {
		return err
	}

	_, err = waitUntilNotRunning(p)
	return err
}

func waitUntilNotRunning(p *process.Process) (bool, error) {
	isRunning, err := p.IsRunning()
	if err != nil {
		return false, err
	}

	tick := time.NewTicker(10 * time.Millisecond)

	for i := 0; i < 100; i++ {
		if !isRunning {
			return true, nil
		}
		<-tick.C
		isRunning, err = p.IsRunning()
		if err != nil {
			return false, err
		}
	}

	return false, nil
}
