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

package exec

import (
	"context"
	"errors"
	"fmt"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
)

// options represents the exec command options
type options struct {
	devSelector       devSelector
	devName           string
	command           []string
	firstArgIsDevName bool
}

type devSelector interface {
	AskForOptionsOkteto(options []utils.SelectorItem, initialPosition int) (string, error)
}

var (
	// errorDevNameRequired is the error returned when the dev name is required
	errDevNameRequired = errors.New("dev name is required")

	// errorCommandRequired is the error returned when the command is required
	errCommandRequired = errors.New("command is required")

	// errNoDevContainerInDevMode is the error returned when there are no development containers in dev mode
	errNoDevContainerInDevMode = errors.New("there are no development containers in dev mode")
)

type errDevNotInManifest struct {
	devName string
}

// Error returns the error message
func (e *errDevNotInManifest) Error() string {
	return fmt.Sprintf("'%s' is not defined in your okteto manifest", e.devName)
}

// newOptions creates a new exec options instance
func newOptions(argsIn []string, argsLenAtDash int) (*options, error) {
	opts := &options{
		command:     []string{},
		devSelector: utils.NewOktetoSelector("Select which development container to exec:", "Development container"),
	}
	if len(argsIn) > 0 && argsLenAtDash != 0 {
		opts.devName = argsIn[0]
		opts.firstArgIsDevName = true
	}
	if argsLenAtDash > -1 {
		opts.command = argsIn[argsLenAtDash:]
	}
	if len(opts.command) == 0 {
		return nil, errCommandRequired
	}
	return opts, nil
}

func (o *options) setDevFromManifest(ctx context.Context, devs model.ManifestDevs, ns string, k8sClientProvider okteto.K8sClientProvider, ioControl *io.Controller) error {
	if o.devName != "" {
		ioControl.Logger().Infof("dev name is already set to '%s'", o.devName)
		return nil
	}
	ioControl.Logger().Debug("retrieving dev name from manifest")

	k8sClient, _, err := k8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		return fmt.Errorf("failed to get k8s client: %w", err)
	}

	devNameList := apps.ListDevModeOn(ctx, devs, ns, k8sClient)
	if len(devNameList) == 0 {
		return errNoDevContainerInDevMode
	}

	devName, err := o.devSelector.AskForOptionsOkteto(utils.ListToSelectorItem(devNameList), -1)
	if err != nil {
		return fmt.Errorf("failed to select dev: %w", err)
	}
	o.devName = devName
	return nil
}

func (o *options) validate(devs model.ManifestDevs) error {
	if o.devName == "" {
		return errDevNameRequired
	}
	if _, ok := devs[o.devName]; !ok {
		return &errDevNotInManifest{devName: o.devName}
	}
	return nil
}
