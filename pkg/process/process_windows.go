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

//go:build windows
// +build windows

package process

import (
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

func (p *Process) Kill() error {
	err := p.Find()
	if err != nil {
		oktetoLog.Debugf("process not running: %d", p.pid)
		return nil
	}
	return p.process.Kill()
}
