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

package destroy

import (
	"context"
	"errors"
	"os"
	"os/signal"

	"github.com/okteto/okteto/pkg/deployable"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

type runner interface {
	RunDestroy(params deployable.DestroyParameters) error
	CleanUp(err error)
}

type localDestroyCommand struct {
	runner runner
}

func newLocalDestroyer(
	runner runner,
) *localDestroyCommand {
	return &localDestroyCommand{
		runner,
	}
}

// Destroy starts the execution of the local destroyer
func (ld *localDestroyCommand) Destroy(_ context.Context, opts *Options) error {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	go func() {
		if opts.Manifest.Destroy == nil {
			exit <- nil
			return
		}

		params := deployable.DestroyParameters{
			Name:         opts.Name,
			Namespace:    opts.Namespace,
			ForceDestroy: opts.ForceDestroy,
			Deployable: deployable.Entity{
				Commands: opts.Manifest.Destroy.Commands,
			},
			Variables: opts.Variables,
		}
		if err := ld.runner.RunDestroy(params); err != nil {
			exit <- err
			return
		}
		exit <- nil
	}()
	select {
	case <-stop:
		oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
		errStop := "interrupt signal received"
		ld.CleanUp(errors.New(errStop))
		return oktetoErrors.ErrIntSig
	case err := <-exit:
		if err != nil {
			oktetoLog.Infof("exit signal received due to error: %s", err)
			return err
		}
	}

	return nil
}

// CleanUp cleans up resources for the runner
func (ld *localDestroyCommand) CleanUp(err error) {
	ld.runner.CleanUp(err)
}
