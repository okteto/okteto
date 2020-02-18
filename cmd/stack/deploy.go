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

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/spf13/cobra"
	"helm.sh/helm/pkg/kube"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
)

//Deploy deploys a stack
func Deploy(ctx context.Context) *cobra.Command {
	var stackPath string
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
	cmd.Flags().StringVarP(&stackPath, "file", "f", "okteto-stack.yaml", "path to the stack manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the deploy command is executed")
	return cmd
}

func executeDeployStack(ctx context.Context) error {
	chartPath := "/Users/pablo/github.com/okteto/stack/chart"
	chart, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf"error loading chart: %s", err)
	}

	kubeconfigPath := "/Users/pablo/.kube/config"
	releaseName := "test"
	releaseNamespace := "pchico83"
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(kube.GetConfig(kubeconfigPath, "", releaseNamespace), releaseNamespace, os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		fmt.Sprintf(format, v)
	}); err != nil {
		panic(err)
	}

	iCli := action.NewInstall(actionConfig)
	iCli.Namespace = releaseNamespace
	iCli.ReleaseName = releaseName
	rel, err := iCli.Run(chart, nil)
	if err != nil {
		panic(err)
	}
	fmt.Println("Successfully installed release: ", rel.Name)
	return nil
}
