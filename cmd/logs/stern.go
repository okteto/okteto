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

package logs

import (
	"fmt"
	"os"
	"regexp"
	"text/template"
	"time"

	"github.com/fatih/color"
	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stern/stern/stern"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/utils/pointer"
)

func getSternConfig(manifest *model.Manifest, o *Options, kubeconfigFile string) (*stern.Config, error) {
	location, err := time.LoadLocation("Local")
	if err != nil {
		return nil, err
	}

	includePodQuery, err := regexp.Compile(o.Include)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regular expression from query: %w", err)
	}
	var excludePodQuery *regexp.Regexp
	if o.exclude != "" {
		excludePodQuery, err = regexp.Compile(o.exclude)
		if err != nil {
			return nil, fmt.Errorf("failed to compile regular expression for excluded pod query: %w", err)
		}
	}

	labelSelector := labels.NewSelector()
	if !o.All {
		req, err := labels.NewRequirement(model.DeployedByLabel, selection.Equals, []string{format.ResourceK8sMetaString(manifest.Name)})
		if err != nil {
			return nil, err
		}
		labelSelector = labelSelector.Add(*req)
	}
	req, err := labels.NewRequirement(model.InteractiveDevLabel, selection.DoesNotExist, nil)
	if err != nil {
		return nil, err
	}
	labelSelector = labelSelector.Add(*req)

	funs := map[string]interface{}{
		"color": func(color color.Color, text string) string {
			return color.SprintFunc()(text)
		},
	}
	t := "{{color .PodColor .PodName}} {{color .ContainerColor .ContainerName}} {{.Message}}\n"
	tmpl, err := template.New("logs").Funcs(funs).Parse(t)
	if err != nil {
		return nil, err
	}

	containerQuery, err := regexp.Compile(".*")
	if err != nil {
		return nil, fmt.Errorf("failed to compile regular expression for container query: %w", err)
	}
	include, err := regexp.Compile(".*")
	if err != nil {
		return nil, fmt.Errorf("failed to compile regular expression for include: %w", err)
	}
	containerStates := []stern.ContainerState{"running"}
	fieldSelector := fields.Everything()

	return &stern.Config{
		KubeConfig:          kubeconfigFile,
		ContextName:         okteto.UrlToKubernetesContext(okteto.GetContext().Name),
		Namespaces:          []string{manifest.Namespace},
		PodQuery:            includePodQuery,
		ExcludePodQuery:     excludePodQuery,
		ContainerQuery:      containerQuery,
		Include:             []*regexp.Regexp{include},
		InitContainers:      false,
		EphemeralContainers: false,
		Since:               o.Since,
		Template:            tmpl,
		ContainerStates:     containerStates,
		Location:            location,
		LabelSelector:       labelSelector,
		FieldSelector:       fieldSelector,
		TailLines:           pointer.Int64(o.Tail),
		Follow:              true,
		Timestamps:          o.Timestamps,
		AllNamespaces:       false,
		ErrOut:              os.Stderr,
		Out:                 os.Stdout,
	}, nil
}
