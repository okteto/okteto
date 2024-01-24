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
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/repository"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
)

type listFlags struct {
	context   string
	namespace string
	output    string
	labels    []string
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
		Args:  utils.NoArgsAccepted(""),
		RunE: func(cmd *cobra.Command, args []string) error {
			return pipelineListCommandHandler(ctx, flags, contextCMD.NewContextCommand().Run)
		},
	}

	cmd.Flags().StringVarP(&flags.context, "context", "c", "", "context where the pipelines are deployed (defaults to the current context)")
	cmd.Flags().StringArrayVarP(&flags.labels, "label", "", []string{}, "tag and organize dev environments using labels (multiple --label flags accepted)")
	cmd.Flags().StringVarP(&flags.namespace, "namespace", "n", "", "namespace where the pipelines are deployed (defaults to the current namespace)")
	cmd.Flags().StringVarP(&flags.output, "output", "o", "", "output format. One of: ['json', 'yaml']")
	return cmd
}

// pipelineListCommandHandler prepares the right okteto context depending on the provided flags and then calls the actual function that lists pipelines
func pipelineListCommandHandler(ctx context.Context, flags *listFlags, initOkCtx initOkCtxFn) error {
	ctxResource := &model.ContextResource{}
	ctxOptions := &contextCMD.Options{
		Show: false,
	}
	if flags.output == "" {
		ctxOptions.Show = true
	}

	if flags.context != "" {
		if err := ctxResource.UpdateContext(flags.context); err != nil {
			return err
		}
	}

	if flags.namespace != "" {
		if err := ctxResource.UpdateNamespace(flags.namespace); err != nil {
			return err
		}
	}

	ctxOptions.Context = ctxResource.Context
	ctxOptions.Namespace = ctxResource.Namespace

	if err := initOkCtx(ctx, ctxOptions); err != nil {
		return err
	}

	okCtx := okteto.GetContext()

	if !okCtx.IsOkteto {
		return oktetoErrors.ErrContextIsNotOktetoCluster
	}

	if flags.namespace == "" {
		flags.namespace = okCtx.Namespace
	}

	pc, err := NewCommand()
	if err != nil {
		return err
	}
	c, _, err := pc.k8sClientProvider.Provide(okCtx.Cfg)
	if err != nil {
		return fmt.Errorf("failed to load okteto context '%s': %w", okCtx.Name, err)
	}

	return executeListPipelines(ctx, *flags, configmaps.List, getPipelineListOutput, c, os.Stdout)
}

type initOkCtxFn func(ctx context.Context, ctxOptions *contextCMD.Options) error
type getPipelineListOutputFn func(ctx context.Context, listPipelines listPipelinesFn, namespace, labelSelector string, c kubernetes.Interface) ([]pipelineListItem, error)
type listPipelinesFn func(ctx context.Context, namespace, labelSelector string, c kubernetes.Interface) ([]apiv1.ConfigMap, error)

// executeListPipelines is responsible for output management and calling the function that lists pipelines
func executeListPipelines(ctx context.Context, opts listFlags, listPipelines listPipelinesFn, getPipelineListOutput getPipelineListOutputFn, c kubernetes.Interface, w io.Writer) error {
	labelSelector, err := getLabelSelector(opts.labels)
	if err != nil {
		return err
	}

	pipelineListOutput, err := getPipelineListOutput(ctx, listPipelines, opts.namespace, labelSelector, c)
	if err != nil {
		return err
	}
	switch opts.output {
	case "json":
		bytes, err := json.MarshalIndent(pipelineListOutput, "", " ")
		if err != nil {
			return err
		}
		jsonOutput := string(bytes)
		if jsonOutput == "null" {
			jsonOutput = "[]"
		}
		fmt.Fprint(w, jsonOutput)
	case "yaml":
		bytes, err := yaml.Marshal(pipelineListOutput)
		if err != nil {
			return err
		}
		fmt.Fprint(w, string(bytes))
	default:
		tw := tabwriter.NewWriter(w, 1, 1, 2, ' ', 0)
		cols := []string{"Name", "Status", "Repository", "Branch", "Labels"}
		header := strings.Join(cols, "\t")
		fmt.Fprintln(tw, header)
		for _, pipeline := range pipelineListOutput {
			labels := "-"
			if len(pipeline.Labels) > 0 {
				labels = strings.Join(pipeline.Labels, ", ")
			}
			output := fmt.Sprintf("%s\t%s\t%s\t%s\t%s", pipeline.Name, pipeline.Status, pipeline.Repository, pipeline.Branch, labels)
			fmt.Fprintln(tw, output)
		}
		tw.Flush()
	}
	return nil
}

func getLabelSelector(labels []string) (string, error) {
	var labelSelector = []string{
		model.GitDeployLabel,
	}
	for _, label := range labels {
		if label == "" {
			return "", fmt.Errorf("invalid label: the provided label is empty")
		}
		errs := validation.IsValidLabelValue(label)
		if len(errs) > 0 {
			return "", fmt.Errorf("invalid label '%s': %w", label, errors.New(errs[0]))
		}
		labelSelector = append(labelSelector, fmt.Sprintf("%s/%s", constants.EnvironmentLabelKeyPrefix, label))
	}
	return strings.Join(labelSelector, ","), nil
}

func getNamespaceStatus(ctx context.Context, namespace string, c kubernetes.Interface) (string, error) {
	n, err := c.CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	return n.Labels[constants.NamespaceStatusLabel], nil
}

func getPipelineListOutput(ctx context.Context, listPipelines listPipelinesFn, namespace, labelSelector string, c kubernetes.Interface) ([]pipelineListItem, error) {
	// We retrieve the namespace because when the namespace is sleeping, all pipelines are sleeping
	nsStatus, err := getNamespaceStatus(ctx, namespace, c)
	if err != nil {
		return nil, err
	}

	cmList, err := listPipelines(ctx, namespace, labelSelector, c)
	if err != nil {
		return nil, err
	}

	var outputList []pipelineListItem
	for _, cm := range cmList {
		item := pipelineListItem{
			Name:       cm.Data["name"],
			Status:     cm.Data["status"],
			Repository: repository.NewRepository(cm.Data["repository"]).GetAnonymizedRepo(),
			Branch:     cm.Data["branch"],
			Labels:     []string{},
		}

		// if the namespace is sleeping, all pipelines are sleeping
		if nsStatus == "Sleeping" {
			item.Status = nsStatus
		}

		for k, v := range cm.ObjectMeta.Labels {
			prefix := fmt.Sprintf("%s/", constants.EnvironmentLabelKeyPrefix)
			if strings.HasPrefix(k, prefix) && v == "true" {
				label := strings.TrimPrefix(k, prefix)
				item.Labels = append(item.Labels, label)
			}
		}

		outputList = append(outputList, item)
	}

	return outputList, nil
}
