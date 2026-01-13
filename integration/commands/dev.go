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

package commands

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"sync"
	"time"

	ps "github.com/mitchellh/go-ps"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model"
)

// UpOptions has the options for okteto up command
type UpOptions struct {
	Name         string
	Namespace    string
	Service      string
	ManifestPath string
	Workdir      string
	OktetoHome   string
	Token        string
	Deploy       bool
}

// UpCommandProcessResult has the information about the command process
type UpCommandProcessResult struct {
	WaitGroup *sync.WaitGroup
	ErrorChan chan error
	Pid       *os.Process
	Output    bytes.Buffer
}

// RunOktetoUp runs an okteto up command
func RunOktetoUp(oktetoPath string, upOptions *UpOptions) (*UpCommandProcessResult, error) {
	var wg sync.WaitGroup
	upErrorChannel := make(chan error, 1)

	cmd := getUpCmd(oktetoPath, upOptions)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	log.Printf("Running up command: %s", cmd.String())
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("okteto up failed to start: %w", err)
	}

	wg.Add(1)

	go func() {
		defer wg.Done()
		if err := cmd.Wait(); err != nil {
			if err != nil {
				log.Printf("okteto up exited: %s.\nOutput:\n%s", err.Error(), out.String())
				upErrorChannel <- fmt.Errorf("okteto up exited before completion")
			}
		}
	}()

	if err := waitForReady(upOptions.Namespace, upOptions.Name, upOptions.OktetoHome, upErrorChannel); err != nil {
		return nil, err
	}

	return &UpCommandProcessResult{
		WaitGroup: &wg,
		ErrorChan: upErrorChannel,
		Pid:       cmd.Process,
		Output:    out,
	}, nil
}

func RunOktetoUpAndWaitWithOutput(oktetoPath string, upOptions *UpOptions) (bytes.Buffer, error) {
	cmd := getUpCmd(oktetoPath, upOptions)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	log.Printf("Running up command: %s", cmd.String())
	if err := cmd.Start(); err != nil {
		return out, fmt.Errorf("okteto up failed to start: %w", err)
	}

	err := cmd.Wait()
	if err != nil {
		log.Printf("okteto up failed: %v", err)
		log.Printf("okteto up output err: \n%s", out.String())
		return out, err
	}
	return out, nil
}

func getUpCmd(oktetoPath string, upOptions *UpOptions) *exec.Cmd {
	cmd := exec.Command(oktetoPath, "up")
	cmd.Env = os.Environ()
	if upOptions.ManifestPath != "" {
		cmd.Args = append(cmd.Args, "-f", upOptions.ManifestPath)
	}
	if upOptions.Namespace != "" {
		cmd.Args = append(cmd.Args, "-n", upOptions.Namespace)
	}
	if upOptions.Workdir != "" {
		cmd.Dir = upOptions.Workdir
	}
	if upOptions.Service != "" {
		cmd.Args = append(cmd.Args, upOptions.Service)
	}
	if upOptions.Deploy {
		cmd.Args = append(cmd.Args, "--deploy")
	}
	if v := os.Getenv(model.OktetoURLEnvVar); v != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoURLEnvVar, v))
	}

	if upOptions.OktetoHome != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", constants.OktetoHomeEnvVar, upOptions.OktetoHome))
	}
	if upOptions.Token != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoTokenEnvVar, upOptions.Token))
	}

	return cmd
}

// DownOptions has the options for okteto down command
type DownOptions struct {
	Namespace    string
	ManifestPath string
	Workdir      string
	OktetoHome   string
	Token        string
	Service      string
}

// RunOktetoDown runs an okteto down command
func RunOktetoDown(oktetoPath string, downOpts *DownOptions) error {
	downCMD := exec.Command(oktetoPath, "down", "-v")
	if downOpts.ManifestPath != "" {
		downCMD.Args = append(downCMD.Args, "-f", downOpts.ManifestPath)
	}
	if downOpts.Namespace != "" {
		downCMD.Args = append(downCMD.Args, "-n", downOpts.Namespace)
	}
	if downOpts.Workdir != "" {
		downCMD.Dir = downOpts.Workdir
	}
	if v := os.Getenv(model.OktetoURLEnvVar); v != "" {
		downCMD.Env = append(downCMD.Env, fmt.Sprintf("%s=%s", model.OktetoURLEnvVar, v))
	}
	if downOpts.Token != "" {
		downCMD.Env = append(downCMD.Env, fmt.Sprintf("%s=%s", model.OktetoTokenEnvVar, downOpts.Token))
	}
	if downOpts.OktetoHome != "" {
		downCMD.Env = append(downCMD.Env, fmt.Sprintf("%s=%s", constants.OktetoHomeEnvVar, downOpts.OktetoHome))
	}
	if downOpts.Service != "" {
		downCMD.Args = append(downCMD.Args, downOpts.Service)
	}
	downCMD.Env = os.Environ()
	o, err := downCMD.CombinedOutput()

	log.Printf("okteto down output:\n%s", string(o))
	if err != nil {
		m, err := os.ReadFile(downOpts.ManifestPath)
		if err != nil {
			return fmt.Errorf("okteto down failed: %w", err)
		}
		log.Printf("manifest: \n%s\n", string(m))
		return fmt.Errorf("okteto down failed: %w", err)
	}

	return nil
}

// HasUpCommandFinished checks if up command has finished correctly
func HasUpCommandFinished(pid int) bool {
	var err error
	ticker := time.NewTicker(1 * time.Second)
	to := time.NewTicker(15 * time.Second)
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
				err = fmt.Errorf("error when finding process: %w", err)
			} else if found != nil {
				err = fmt.Errorf("okteto up didn't exit after down")
			}
		}
	}
}

func waitForReady(namespace, name, oktetoHome string, upErrorChannel chan error) error {
	log.Println("waiting for okteto up to be ready")

	state := path.Join(oktetoHome, ".okteto", namespace, name, "okteto.state")

	ticker := time.NewTicker(1 * time.Second)
	to := time.NewTicker(300 * time.Second)
	retry := 0
	for {
		select {
		case <-upErrorChannel:
			return fmt.Errorf("okteto up failed")
		case <-to.C:
			return fmt.Errorf("development container was never ready")
		case <-ticker.C:
			retry++
			c, err := os.ReadFile(state)
			if err != nil {
				if retry%10 == 0 {
					log.Printf("failed to read state file %s: %s", state, err)
				}
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
			} else if retry%10 == 0 {
				log.Printf("okteto up is: %s", c)
			}
		}
	}

}
