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
	"strings"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/helm"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/pkg/errors"
	yaml "gopkg.in/yaml.v2"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/repo"
)

//Deploy deploys a stack
func Deploy(ctx context.Context, s *model.Stack, forceBuild, wait bool) error {
	if s.Namespace != "" {
		os.Setenv("HELM_NAMESPACE", s.Namespace)
	}
	settings := cli.New()
	if s.Namespace == "" {
		s.Namespace = settings.Namespace()
	}

	if err := translate(s, forceBuild); err != nil {
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
			if r.Name != stackHelmRepoName {
				continue
			}
			re = r
			break
		}
	}
	if re == nil {
		if err := helm.AddRepo(settings, stackHelmRepoName, stackHelmRepoURL, stackHelmChartName, stackHelmChartVersion); err != nil {
			return err
		}
		log.Information("'%s' has been added to your helm repositories.", stackHelmRepoName)
	} else {
		if err := helm.UpdateRepo(re, settings, stackHelmChartName); err != nil {
			return err
		}
	}

	spinner := utils.NewSpinner(fmt.Sprintf("Deploying stack '%s'...", s.Name))
	spinner.Start()
	defer spinner.Stop()

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), s.Namespace, helmDriver, func(format string, v ...interface{}) {
		message := strings.TrimSuffix(fmt.Sprintf(format, v...), "\n")
		spinner.Update(fmt.Sprintf("%s...", message))
	}); err != nil {
		return fmt.Errorf("error initializing stack client: %s", err)
	}

	exists, err := helm.ReleaseExist(action.NewList(actionConfig), s.Name)
	if err != nil {
		return fmt.Errorf("error listing stacks: %s", err)
	}
	if exists {
		return helm.Upgrade(action.NewUpgrade(actionConfig), settings, s, stackHelmRepoName, stackHelmChartName, stackHelmChartVersion, vals, wait)
	}
	return helm.Install(action.NewInstall(actionConfig), settings, s, stackHelmRepoName, stackHelmChartName, stackHelmChartVersion, vals, wait)
}

func isNotExist(err error) bool {
	return os.IsNotExist(errors.Cause(err))
}

func saveStackFile(s *model.Stack) (string, error) {
	tmpFile, err := ioutil.TempFile("", "okteto-stack")
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
