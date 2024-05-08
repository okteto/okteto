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

package ssh

import (
	"os"
	"os/signal"
	"syscall"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

func resizeWindow(session *ssh.Session) {
	resize := make(chan os.Signal, 1)
	signal.Notify(resize, syscall.SIGWINCH)
	go func() {
		for range resize {
			width, height, err := term.GetSize(int(os.Stdout.Fd()))
			oktetoLog.Infof("terminal width %d height %d", width, height)
			if err != nil {
				oktetoLog.Infof("request for terminal size failed: %s", err)
			}
			if err := session.WindowChange(height, width); err != nil {
				oktetoLog.Infof("request for terminal resize failed: %s", err)
			}
		}
	}()
}
