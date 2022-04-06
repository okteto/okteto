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

package up

import (
	"fmt"
	"io"
	"runtime"
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/config"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/shirou/gopsutil/process"
)

func (up *upContext) stopRunningUpOnSameFolder() error {
	pList, err := process.Processes()
	if err != nil {
		return err
	}
	folderName := config.GetAppHome(up.Dev.Namespace, up.Dev.Name)
	for _, p := range pList {
		if p.Pid == 0 {
			continue
		}

		name, err := p.Name()
		if err != nil {
			// it's expected go get EOF if the process no longer exists at this point.
			if err != io.EOF {
				oktetoLog.Infof("error getting name for process %d: %s", p.Pid, err.Error())
			}
			continue
		}

		if name == "" {
			oktetoLog.Infof("ignoring pid %d with no name: %v", p.Pid, p)
			continue
		}

		if !strings.Contains(name, "syncthing") {
			continue
		}

		cmdline, err := p.Cmdline()
		if err != nil {
			return err
		}

		oktetoLog.Infof("checking syncthing home '%s' with command '%s'", folderName, cmdline)
		if strings.Contains(cmdline, fmt.Sprintf("-home %s", folderName)) {
			parent, err := getParent(p)
			if err != nil {
				oktetoLog.Info("can not find parent")
				continue
			}

			if err := terminate(parent, true); err != nil {
				oktetoLog.Infof("error terminating syncthing %d with wait: %s", p.Pid, err.Error())
				continue
			}

			continue
		}

	}
	return nil
}

func getParent(p *process.Process) (*process.Process, error) {
	name, err := p.Name()
	if err != nil {
		return nil, fmt.Errorf("can not get parent name")
	}
	parent, err := p.Parent()
	if err != nil {
		return nil, fmt.Errorf("can not find parent")
	}
	if parent.Pid < 100 {
		return nil, fmt.Errorf("can't remove root process")
	}
	pName, err := parent.Name()
	if pName == name {
		return getParent(parent)
	}
	return parent, nil
}

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
