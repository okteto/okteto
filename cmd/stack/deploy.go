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
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/helm"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v2"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/repo"
)

//Deploy deploys a stack
func Deploy(ctx context.Context) *cobra.Command {
	var stackPath string
	var namespace string
	var forceBuild bool
	var wait bool
	cmd := &cobra.Command{
		Use:   "deploy <name>",
		Short: fmt.Sprintf("Deploys a stack"),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := utils.LoadStack(args[0], stackPath)
			if err != nil {
				return err
			}

			if err := s.UpdateNamespace(namespace); err != nil {
				return err
			}
			err = executeDeployStack(ctx, s, forceBuild, wait)
			analytics.TrackDeployStack(err == nil)
			if err == nil {
				log.Success("Successfully deployed stack '%s'", s.Name)
			}
			return err
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("deploy requires the stack NAME argument")
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&stackPath, "file", "f", utils.DefaultStackManifest, "path to the stack manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "overwrites the stack namespace where the stack is deployed")
	cmd.Flags().BoolVarP(&forceBuild, "build", "", false, "build images before starting containers")
	cmd.Flags().BoolVarP(&wait, "wait", "", false, "wait until a minimum number of Pods of a Deployment, StatefulSet are in a ready state")
	return cmd
}

func executeDeployStack(ctx context.Context, s *model.Stack, forceBuild, wait bool) error {
	settings := cli.New()
	if s.Namespace == "" {
		s.Namespace = settings.Namespace()
	}

	if err := helm.Translate(s, forceBuild); err != nil {
		return err
	}

	dynamicStackFilename, err := saveStackFile(s)
	if err != nil {
		return err
	}
	defer os.Remove(dynamicStackFilename)

	valueOpts := &values.Options{}
	valueOpts.ValueFiles = []string{dynamicStackFilename}
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
		if err := helm.RepoUpdate(re, settings, HelmChartName); err != nil {
			return err
		}
	}

	spinner := utils.NewSpinner(fmt.Sprintf("Deploying stack '%s'...", s.Name))
	spinner.Start()
	defer spinner.Stop()

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), s.Namespace, helm.HelmDriver, func(format string, v ...interface{}) {
		message := fmt.Sprintf(format, v...)
		spinner.Update(fmt.Sprintf("%s...", message))
	}); err != nil {
		return fmt.Errorf("error initializing stack client: %s", err)
	}

	exists, err := helm.ExistRelease(action.NewList(actionConfig), s.Name)
	if err != nil {
		return fmt.Errorf("error listing stacks: %s", err)
	}
	if exists {
		return helm.Upgrade(action.NewUpgrade(actionConfig), settings, s, HelmRepoName, HelmChartName, HelmChartVersion, vals, wait)
	}
	return helm.Install(action.NewInstall(actionConfig), settings, s, HelmRepoName, HelmChartName, HelmChartVersion, vals, wait)
}

func isNotExist(err error) bool {
	return os.IsNotExist(errors.Cause(err))
}

func saveStackFile(s *model.Stack) (string, error) {
	stackTmpFolder := filepath.Join(config.GetOktetoHome(), ".stacks")
	if err := os.MkdirAll(stackTmpFolder, 0700); err != nil {
		return "", fmt.Errorf("failed to create %s: %s", stackTmpFolder, err)
	}

	tmpFile, err := ioutil.TempFile(stackTmpFolder, "stack-")
	if err != nil {
		return "", fmt.Errorf("failed to create dynamic stack manifest file: %s", err)
	}

	marshalled, err := yaml.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("failed to marshall dynamic stack manifest: %s", err)
	}

	if err := ioutil.WriteFile(tmpFile.Name(), marshalled, 0600); err != nil {
		return "", fmt.Errorf("failed to save dynaamic stack manifest: %s", err)
	}
	return tmpFile.Name(), nil
}
