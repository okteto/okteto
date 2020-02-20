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
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/helm"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/spf13/cobra"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
)

//Destroy destroys a stack
func Destroy(ctx context.Context) *cobra.Command {
	var stackPath string
	var namespace string
	cmd := &cobra.Command{
		Use:   "destroy",
		Short: fmt.Sprintf("Destroys a stack"),
		RunE: func(cmd *cobra.Command, args []string) error {

			s, err := utils.LoadStack(stackPath)
			if err != nil {
				return err
			}

			if err := s.UpdateNamespace(namespace); err != nil {
				return err
			}

			err = executeDestroyStack(ctx, s)
			analytics.TrackDestroyStack(err == nil)
			if err == nil {
				log.Success("Successfully destroyed stack '%s'", s.Name)
			}
			return err
		},
	}
	cmd.Flags().StringVarP(&stackPath, "file", "f", utils.DefaultStackManifest, "path to the stack manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "overwrites the stack namespace where the stack is destroyed")
	return cmd
}

func executeDestroyStack(ctx context.Context, s *model.Stack) error {
	spinner := utils.NewSpinner(fmt.Sprintf("Destroying stack '%s'...", s.Name))
	spinner.Start()
	defer spinner.Stop()

	settings := cli.New()
	actionConfig := new(action.Configuration)
	if s.Namespace == "" {
		s.Namespace = settings.Namespace()
	}

	if err := actionConfig.Init(settings.RESTClientGetter(), s.Namespace, helm.HelmDriver, func(format string, v ...interface{}) {
		log.Infof(fmt.Sprintf(format, v...))
	}); err != nil {
		return fmt.Errorf("error initializing stack client: %s", err)
	}

	exists, err := helm.ExistRelease(action.NewList(actionConfig), s.Name)
	if err != nil {
		return fmt.Errorf("error listing stacks: %s", err)
	}
	if !exists {
		return fmt.Errorf("stack %s does not exist", s.Name)
	}

	uClient := action.NewUninstall(actionConfig)
	if _, err := uClient.Run(s.Name); err != nil {
		return fmt.Errorf("error destroying stack '%s': %s", s.Name, err)
	}
	return nil
}
