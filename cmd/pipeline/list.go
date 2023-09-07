// Copyright 2023 The Okteto Authors
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

package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	contextCMD "github.com/okteto/okteto/cmd/context"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"os"
	"strings"
	"text/tabwriter"
)

type listFlags struct {
	namespace string
	output    string
}

type pipelineListItem struct {
	Name       string   `json:"name" yaml:"name"`
	Status     string   `json:"status" yaml:"status"`
	Repository string   `json:"repository" yaml:"repository"`
	Branch     string   `json:"branch" yaml:"branch"`
	Labels     []string `json:"labels" yaml:"labels"`
}

func list(ctx context.Context) *cobra.Command {
	flags := &listFlags{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all okteto pipelines",
		//Args:  utils.NoArgsAccepted("https://www.okteto.com/docs/reference/cli/#-1"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.namespace == "" {
				flags.namespace = okteto.Context().Namespace
			}

			ctxResource := &model.ContextResource{}
			if err := ctxResource.UpdateNamespace(flags.namespace); err != nil {
				return err
			}

			ctxOptions := &contextCMD.ContextOptions{
				Namespace: ctxResource.Namespace,
				Show:      true,
			}
			if err := contextCMD.NewContextCommand().Run(ctx, ctxOptions); err != nil {
				return err
			}

			if !okteto.IsOkteto() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}

			pc, err := NewCommand()
			if err != nil {
				return err
			}
			c, _, err := pc.k8sClientProvider.Provide(okteto.Context().Cfg)
			if err != nil {
				return fmt.Errorf("failed to load okteto context '%s': %v", okteto.Context().Name, err)
			}

			return pc.executeListPipelines(cmd.Context(), *flags, configmaps.List, model.GitDeployLabel, c)
		},
	}

	cmd.Flags().StringVarP(&flags.namespace, "namespace", "n", "", "namespace where the pipelines are deployed (defaults to the current namespace)")
	cmd.Flags().StringVarP(&flags.output, "output", "o", "", "output format. One of: ['json', 'yaml']")
	return cmd
}

type listPipelinesFn func(ctx context.Context, namespace, labelSelector string, c kubernetes.Interface) ([]apiv1.ConfigMap, error)

func (pc *Command) executeListPipelines(ctx context.Context, opts listFlags, listPipelines listPipelinesFn, labelSelector string, c kubernetes.Interface) error {
	pipelineListOutput, err := pc.getPipelineListOutput(ctx, listPipelines, opts.namespace, labelSelector, c)
	if err != nil {
		return err
	}
	switch opts.output {
	case "json":
		bytes, err := json.MarshalIndent(pipelineListOutput, "", " ")
		if err != nil {
			return err
		}
		oktetoLog.Println(string(bytes))
	case "yaml":
		bytes, err := yaml.Marshal(pipelineListOutput)
		if err != nil {
			return err
		}
		oktetoLog.Println(string(bytes))
	default:
		w := tabwriter.NewWriter(os.Stdout, 1, 1, 2, ' ', 0)
		cols := []string{"Name", "Status", "Repository", "Branch", "Labels"}
		header := strings.Join(cols, "\t")
		fmt.Fprintln(w, header)
		for _, pipeline := range pipelineListOutput {
			labels := "-"
			if len(pipeline.Labels) > 0 {
				labels = strings.Join(pipeline.Labels, ", ")
			}
			output := fmt.Sprintf("%s\t%s\t%s\t%s\t%s", pipeline.Name, pipeline.Status, pipeline.Repository, pipeline.Branch, labels)
			fmt.Fprintln(w, output)
		}
		w.Flush()
	}
	return nil
}

func (pc *Command) getPipelineListOutput(ctx context.Context, listPipelines listPipelinesFn, namespace, labelSelector string, c kubernetes.Interface) ([]pipelineListItem, error) {
	cmList, err := listPipelines(ctx, namespace, labelSelector, c)
	if err != nil {
		return nil, err
	}

	var outputList []pipelineListItem
	for _, cm := range cmList {
		item := pipelineListItem{
			Name:       cm.Data["name"],
			Status:     cm.Data["status"],
			Repository: cm.Data["repository"],
			Branch:     cm.Data["branch"],
			Labels:     []string{},
		}

		// TODO: filter only actual labels
		//for k, v := range cm.ObjectMeta.Labels {
		//	if v == "true" {
		//		item.Labels = append(item.Labels, k)
		//	}
		//}

		outputList = append(outputList, item)
	}

	return outputList, nil
}
