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
	"errors"
	"fmt"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
)

type Options struct {
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
)

type errDevNotInManifest struct {
	devName string
}

func (e *errDevNotInManifest) Error() string {
	return fmt.Sprintf("'%s' is not defined in your okteto manifest", e.devName)
}

func NewOptions(argsIn []string, argsLenAtDash int) *Options {
	opts := &Options{
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
	return opts
}

func (o *Options) setDevFromManifest(devs model.ManifestDevs, ioControl *io.Controller) error {
	if o.devName != "" {
		ioControl.Logger().Infof("dev name is already set to '%s'", o.devName)
		return nil
	}
	ioControl.Logger().Debug("retrieving dev name from manifest")

	devNameList := devs.GetDevs()
	if len(devNameList) == 1 {
		o.devName = devNameList[0]
		ioControl.Logger().Infof("dev name set to '%s'", o.devName)
		return nil
	}
	devName, err := o.devSelector.AskForOptionsOkteto(utils.ListToSelectorItem(devs.GetDevs()), -1)
	if err != nil {
		return fmt.Errorf("failed to select dev: %w", err)
	}
	o.devName = devName
	return nil
}

func (o *Options) Validate(devs model.ManifestDevs) error {
	if o.devName == "" {
		return errDevNameRequired
	}
	if len(o.command) == 0 {
		return errCommandRequired
	}
	if _, ok := devs[o.devName]; !ok {
		return &errDevNotInManifest{devName: o.devName}
	}
	return nil
}
