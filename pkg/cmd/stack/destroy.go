// Copyright 2020 The Okteto Authors
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

package stack

import (
	"context"
	"fmt"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/helm"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
)

//Destroy destroys a stack
func Destroy(ctx context.Context, name, namespace string) error {
	spinner := utils.NewSpinner(fmt.Sprintf("Destroying stack '%s'...", name))
	spinner.Start()
	defer spinner.Stop()

	settings := cli.New()
	actionConfig := new(action.Configuration)
	if namespace == "" {
		namespace = settings.Namespace()
	}

	if err := actionConfig.Init(settings.RESTClientGetter(), namespace, helmDriver, func(format string, v ...interface{}) {
		message := fmt.Sprintf(format, v...)
		spinner.Update(fmt.Sprintf("%s...", message))
	}); err != nil {
		return fmt.Errorf("error initializing stack client: %s", err)
	}

	exists, err := helm.ReleaseExist(action.NewList(actionConfig), name)
	if err != nil {
		return fmt.Errorf("error listing stacks: %s", err)
	}
	if !exists {
		return fmt.Errorf("stack %s does not exist", name)
	}

	uClient := action.NewUninstall(actionConfig)
	if _, err := uClient.Run(name); err != nil {
		return fmt.Errorf("error destroying stack '%s': %s", name, err)
	}
	return nil
}
