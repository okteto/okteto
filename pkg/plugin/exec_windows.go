// Copyright 2023-2025 The Okteto Authors
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

package plugin

import (
	"errors"
	"os"
	"os/exec"
	"os/signal"
)

// execPlugin runs the plugin as a child process (Windows has no exec
// syscall) and exits with the child's real exit code. Interrupts are
// ignored while the child runs: Ctrl+C reaches the child through the
// console, and the child decides when to stop. It returns only when the
// child could not be started.
func execPlugin(path string, argv []string, environ []string) error {
	c := exec.Command(path, argv[1:]...)
	c.Env = environ
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	signal.Ignore(os.Interrupt)
	err := c.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode()) // skipcq: RVV-A0003
	}
	if err != nil {
		return err
	}
	os.Exit(0) // skipcq: RVV-A0003
	return nil
}
