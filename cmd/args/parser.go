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

package args

import (
	"context"
	"fmt"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
)

type devSelector interface {
	AskForOptionsOkteto(options []utils.SelectorItem, initialPosition int) (string, error)
}

type devLister interface {
	List(ctx context.Context, devs model.ManifestDevs, namespace string) ([]string, error)
}

// DevCommandArgParser is a parser for commands that takes a development container and a command as arguments
type DevCommandArgParser struct {
	devSelector devSelector
	devLister   devLister
	ioCtrl      *io.Controller
}

// NewDevCommandArgParser creates a new DevCommandArgParser instance
func NewDevCommandArgParser(lister devLister, ioControl *io.Controller) *DevCommandArgParser {
	return &DevCommandArgParser{
		devSelector: utils.NewOktetoSelector("Select the development container:", "Development container"),
		ioCtrl:      ioControl,
		devLister:   lister,
	}
}

type Result struct {
	DevName           string
	Command           []string
	FirstArgIsDevName bool
}

// Parse parses the arguments and returns the dev name and command
func (p *DevCommandArgParser) Parse(ctx context.Context, argsIn []string, argsLenAtDash int, devs model.ManifestDevs, ns string) (*Result, error) {
	result, err := p.parseFromArgs(argsIn, argsLenAtDash)
	if err != nil {
		return nil, err
	}
	result, err = p.setDevNameFromManifest(ctx, result, devs, ns)
	if err != nil {
		return nil, err
	}
	if err := p.validate(result, devs); err != nil {
		return nil, err
	}
	return result, nil
}

// parseFromArgs parses the arguments and returns the dev name and command
func (p *DevCommandArgParser) parseFromArgs(argsIn []string, argsLenAtDash int) (*Result, error) {
	result := &Result{}
	if len(argsIn) > 0 && argsLenAtDash != 0 {
		result.DevName = argsIn[0]
		result.FirstArgIsDevName = true
	}
	if argsLenAtDash > -1 {
		result.Command = argsIn[argsLenAtDash:]
	}
	return result, nil
}

// setDevNameFromManifest sets the dev name from the manifest if it is not already set
func (p *DevCommandArgParser) setDevNameFromManifest(ctx context.Context, currentResult *Result, devs model.ManifestDevs, ns string) (*Result, error) {
	if currentResult.DevName != "" {
		p.ioCtrl.Logger().Infof("dev name is already set to '%s'", currentResult.DevName)
		return currentResult, nil
	}
	p.ioCtrl.Logger().Debug("retrieving dev name from manifest")

	devNameList, err := p.devLister.List(ctx, devs, ns)
	if err != nil {
		return nil, fmt.Errorf("failed to list devs: %w", err)
	}

	devName, err := p.devSelector.AskForOptionsOkteto(utils.ListToSelectorItem(devNameList), -1)
	if err != nil {
		return nil, fmt.Errorf("failed to select dev: %w", err)
	}
	currentResult.DevName = devName
	return currentResult, nil
}

// validate validates that the dev name is set and exists in the manifest
func (p *DevCommandArgParser) validate(result *Result, devs model.ManifestDevs) error {
	if result.DevName == "" {
		return errDevNameRequired
	}
	if _, ok := devs[result.DevName]; !ok {
		return &errDevNotInManifest{devName: result.DevName}
	}
	return nil
}
