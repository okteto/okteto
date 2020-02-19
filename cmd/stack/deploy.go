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

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/log"
	"github.com/spf13/cobra"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/kube"
)

//Deploy deploys a stack
func Deploy(ctx context.Context) *cobra.Command {
	var stackPath string
	var name string
	var namespace string
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: fmt.Sprintf("Deploys a stack"),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := executeDeployStack(ctx)
			analytics.TrackDeployStack(err == nil)
			return err
		},
	}
	cmd.Flags().StringVarP(&stackPath, "file", "f", "okteto-stack.yml", "path to the stack manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "overwrites the stack namespace where the stack is deployed")
	cmd.Flags().StringVarP(&name, "name", "", "", "overwrites the stack name")
	return cmd
}

func executeDeployStack(ctx context.Context) error {
	kubeconfigPath := "/Users/pablo/.kube/config"
	releaseName := "test"
	releaseNamespace := "pchico83"
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(kube.GetConfig(kubeconfigPath, "", releaseNamespace), releaseNamespace, "secrets", func(format string, v ...interface{}) {
		log.Infof(fmt.Sprintf(format, v...))
	}); err != nil {
		panic(err)
	}

	valueOpts := &values.Options{}
	valueOpts.ValueFiles = []string{"/Users/pablo/github.com/okteto/stack/okteto-stack.yaml"}
	vals, err := valueOpts.MergeValues(nil)
	if err != nil {
		return err
	}

	lClient := action.NewList(actionConfig)
	lClient.AllNamespaces = false
	results, err := lClient.Run()
	if err != nil {
		return err
	}
	install := true
	for _, release := range results {
		if release.Name == releaseName {
			install = false
			break
		}

	}

	//TODO: download chart and update and check given version is available

	if install {
		iCli := action.NewInstall(actionConfig)
		iCli.Namespace = releaseNamespace
		iCli.ReleaseName = releaseName
		settings := cli.New()
		chartPath, err := iCli.ChartPathOptions.LocateChart("okteto-stack/stack", settings)
		if err != nil {
			return err
		}

		chart, err := loader.Load(chartPath)
		if err != nil {
			return fmt.Errorf("error loading chart: %s", err)
		}

		rel, err := iCli.Run(chart, vals)
		if err != nil {
			panic(err)
		}
		fmt.Println("Successfully installed release: ", rel.Name)
	} else {
		uCli := action.NewUpgrade(actionConfig)
		uCli.Namespace = releaseNamespace
		settings := cli.New()
		chartPath, err := uCli.ChartPathOptions.LocateChart("okteto-stack/stack", settings)
		if err != nil {
			return err
		}

		chart, err := loader.Load(chartPath)
		if err != nil {
			return fmt.Errorf("error loading chart: %s", err)
		}

		rel, err := uCli.Run(releaseName, chart, vals)
		if err != nil {
			panic(err)
		}
		fmt.Println("Successfully upgraded release: ", rel.Name)
	}

	return nil
}
