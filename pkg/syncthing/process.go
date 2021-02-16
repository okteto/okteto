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

package syncthing

import (
	"time"

	"github.com/okteto/okteto/pkg/log"
	"github.com/shirou/gopsutil/process"
)

func getParentExe(p *process.Process) string {
	parent, err := p.Parent()
	if err != nil {
		log.Infof("error getting parent process: %s", err.Error())
		return ""
	}
	if parent == nil {
		return ""
	}
	parentExe, err := parent.Exe()
	if err != nil {
		log.Infof("error getting parent process exe: %s", err.Error())
		return ""
	}
	return parentExe

}

func terminate(p *process.Process, wait bool) error {
	if err := p.Terminate(); err != nil {
		return err
	}

	if wait {
		isRunning, err := p.IsRunning()
		if err != nil {
			return err
		}
		for isRunning {
			time.Sleep(10 * time.Millisecond)
			isRunning, err = p.IsRunning()
			if err != nil {
				return err
			}
		}
	}

	return nil
}
