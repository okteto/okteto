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

package commands

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"sync"
	"time"

	ps "github.com/mitchellh/go-ps"
	"github.com/okteto/okteto/pkg/config"
)

// UpCommandProcessResult has the information about the command process
type UpCommandProcessResult struct {
	WaitGroup sync.WaitGroup
	ErrorChan chan error
	Pid       *os.Process
}

// RunOktetoUp runs an okteto up command
func RunOktetoUp(namespace, name, manifestPath, oktetoPath string) (*UpCommandProcessResult, error) {
	var wg sync.WaitGroup
	upErrorChannel := make(chan error, 1)
	pid, err := up(context.Background(), &wg, namespace, name, manifestPath, oktetoPath, upErrorChannel)
	if err != nil {
		return nil, err
	}
	return &UpCommandProcessResult{
		WaitGroup: wg,
		ErrorChan: upErrorChannel,
		Pid:       pid,
	}, nil
}

// RunOktetoDown runs an okteto down command
func RunOktetoDown(namespace, name, manifestPath, oktetoPath string) error {
	downCMD := exec.Command(oktetoPath, "down", "-n", namespace, "-f", manifestPath, "-v")
	downCMD.Env = os.Environ()
	o, err := downCMD.CombinedOutput()

	log.Printf("okteto down output:\n%s", string(o))
	if err != nil {
		m, _ := os.ReadFile(manifestPath)
		log.Printf("manifest: \n%s\n", string(m))
		return fmt.Errorf("okteto down failed: %s", err)
	}

	return nil
}

// HasUpCommandFinished checks if up command has finished correctly
func HasUpCommandFinished(pid int) bool {
	var err error
	ticker := time.NewTicker(1 * time.Second)
	to := time.NewTicker(30 * time.Second)
	for {
		select {
		case <-to.C:
			log.Print(err)
			return false
		case <-ticker.C:
			var found ps.Process
			found, err = ps.FindProcess(pid)
			if err == nil && found == nil {
				return true
			}

			if err != nil {
				err = fmt.Errorf("error when finding process: %s", err)
			} else if found != nil {
				err = fmt.Errorf("okteto up didn't exit after down")
			}
		}
	}
}

func up(ctx context.Context, wg *sync.WaitGroup, namespace, name, manifestPath, oktetoPath string, upErrorChannel chan error) (*os.Process, error) {
	var out bytes.Buffer
	cmd := exec.Command(oktetoPath, "up", "-n", namespace, "-f", manifestPath)
	cmd.Env = os.Environ()
	cmd.Stdout = &out
	cmd.Stderr = &out
	log.Printf("up command: %s", cmd.String())
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("okteto up failed to start: %s", err)
	}

	wg.Add(1)

	go func() {
		defer wg.Done()
		if err := cmd.Wait(); err != nil {
			if err != nil {
				log.Printf("okteto up exited: %s.\nOutput:\n%s", err, out.String())
				upErrorChannel <- fmt.Errorf("Okteto up exited before completion")
			}
		}
	}()

	return cmd.Process, waitForReady(namespace, name, upErrorChannel)
}

func waitForReady(namespace, name string, upErrorChannel chan error) error {
	log.Println("waiting for okteto up to be ready")

	state := path.Join(config.GetOktetoHome(), namespace, name, "okteto.state")

	ticker := time.NewTicker(1 * time.Second)
	to := time.NewTicker(300 * time.Second)
	for {
		select {
		case <-to.C:
			return fmt.Errorf("development container was never ready")
		case <-ticker.C:
			c, err := os.ReadFile(state)
			if err != nil {
				log.Printf("failed to read state file %s: %s", state, err)
				if !os.IsNotExist(err) {
					return err
				}
				continue
			}

			if string(c) == "ready" {
				log.Printf("okteto up is: %s", c)
				return nil
			} else if string(c) == "failed" {
				return fmt.Errorf("development container failed")
			}

		}
	}

}
