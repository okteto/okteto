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
	"os"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/helm"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/repo"
)

//Deploy deploys a stack
func Deploy(ctx context.Context) *cobra.Command {
	var stackPath string
	var namespace string
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: fmt.Sprintf("Deploys a stack"),
		RunE: func(cmd *cobra.Command, args []string) error {

			s, err := utils.LoadStack(stackPath)
			if err != nil {
				return err
			}

			if err := s.UpdateNamespace(namespace); err != nil {
				return err
			}
			err = executeDeployStack(ctx, s, stackPath)
			analytics.TrackDeployStack(err == nil)
			if err == nil {
				log.Success("Successfully deployed stack '%s'", s.Name)
			}
			return err
		},
	}
	cmd.Flags().StringVarP(&stackPath, "file", "f", utils.DefaultStackManifest, "path to the stack manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "overwrites the stack namespace where the stack is deployed")
	return cmd
}

func executeDeployStack(ctx context.Context, s *model.Stack, stackPath string) error {
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

	valueOpts := &values.Options{}
	valueOpts.ValueFiles = []string{stackPath}
	vals, err := valueOpts.MergeValues(nil)
	if err != nil {
		return fmt.Errorf("error initializing stack values: %s", err)
	}

	var re *repo.Entry
	rf, err := repo.LoadFile(settings.RepositoryConfig)
	if !isNotExist(err) {
		for _, r := range rf.Repositories {
			if r.Name != HelmRepoName {
				continue
			}
			re = r
			break
		}
	}
	if re == nil {
		if err := helm.RepoAdd(settings, HelmRepoName, HelmRepoURL, HelmChartName, HelmChartVersion); err != nil {
			return err
		}
		log.Information("'%s' has been added to your helm repositories.", HelmRepoName)
	} else {
		if err := helm.RepoUpdate(re, settings, HelmRepoName, HelmChartName, HelmChartVersion); err != nil {
			return err
		}
	}

	spinner := utils.NewSpinner(fmt.Sprintf("Deploying stack '%s'...", s.Name))
	spinner.Start()
	defer spinner.Stop()

	exists, err := helm.ExistRelease(action.NewList(actionConfig), s.Name)
	if err != nil {
		return fmt.Errorf("error listing stacks: %s", err)
	}
	if exists {
		return helm.Upgrade(action.NewUpgrade(actionConfig), settings, s, HelmRepoName, HelmChartName, HelmChartVersion, vals)
	}
	return helm.Install(action.NewInstall(actionConfig), settings, s, HelmRepoName, HelmChartName, HelmChartVersion, vals)
}

func isNotExist(err error) bool {
	return os.IsNotExist(errors.Cause(err))
}
