// Copyright 2022 The Okteto Authors
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

package up

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"text/template"
	"time"

	"github.com/fatih/color"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/pkg/errors"
	"github.com/stern/stern/stern"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/clientcmd"
)

var tempKubeConfigTemplate = "%s/.okteto/kubeconfig-%s"

func (up *upContext) showStackLogs(ctx context.Context) error {
	tmpKubeconfig, err := createTempKubeconfig(up.Manifest.Name)
	if err != nil {
		return err
	}
	defer os.Remove(tmpKubeconfig)

	c, err := getSternConfig(tmpKubeconfig, fmt.Sprintf("stack.okteto.com/name=%s", up.Manifest.Name))
	if err != nil {
		return err
	}
	exit := make(chan error, 1)
	go func() {
		if err := stern.Run(ctx, c); err != nil {
			exit <- oktetoErrors.UserError{
				E: fmt.Errorf("failed to get logs: %w", err),
			}
		}
	}()
	select {
	case <-ctx.Done():
		oktetoLog.Infof("showDetachedLogs context cancelled")
		return ctx.Err()
	case err := <-exit:
		if err != nil {
			oktetoLog.Infof("exit signal received due to error: %s", err)
			return err
		}
	}
	return nil
}

func (up *upContext) showDetachedLogs(ctx context.Context) error {
	tmpKubeconfig, err := createTempKubeconfig(up.Manifest.Name)
	if err != nil {
		return err
	}
	defer os.Remove(tmpKubeconfig)

	c, err := getSternConfig(tmpKubeconfig, "detached.dev.okteto.com")
	if err != nil {
		return err
	}
	exit := make(chan error, 1)
	go func() {
		if err := stern.Run(ctx, c); err != nil {
			exit <- oktetoErrors.UserError{
				E: fmt.Errorf("failed to get logs: %w", err),
			}
		}
	}()
	select {
	case <-ctx.Done():
		oktetoLog.Infof("showDetachedLogs context cancelled")
		return ctx.Err()
	case err := <-exit:
		if err != nil {
			oktetoLog.Infof("exit signal received due to error: %s", err)
			return err
		}
	}
	return nil
}

func createTempKubeconfig(name string) (string, error) {
	cfg := okteto.Context().Cfg
	destKubeconfigFile := fmt.Sprintf(tempKubeConfigTemplate, config.GetUserHomeDir(), name)
	if err := clientcmd.WriteToFile(*cfg, destKubeconfigFile); err != nil {
		oktetoLog.Errorf("could not modify the k8s config: %s", err)
		return "", err
	}
	return destKubeconfigFile, nil
}

func getSternConfig(kubeconfigPath, labelSelectorString string) (*stern.Config, error) {
	labelSelector, err := labels.Parse(labelSelectorString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse selector as label selector: %s", err.Error())
	}
	pod, err := regexp.Compile("")
	if err != nil {
		return nil, fmt.Errorf("failed to compile regular expression from query: %w", err)
	}
	container, err := regexp.Compile(".*")
	if err != nil {
		return nil, errors.Wrap(err, "failed to compile regular expression for container query")
	}
	var tailLines *int64

	funs := template.FuncMap{
		"json": func(in interface{}) (string, error) {
			b, err := json.Marshal(in)
			if err != nil {
				return "", err
			}
			return string(b), nil
		},
		"parseJSON": func(text string) (map[string]interface{}, error) {
			obj := make(map[string]interface{})
			if err := json.Unmarshal([]byte(text), &obj); err != nil {
				return obj, err
			}
			return obj, nil
		},
		"color": func(color color.Color, text string) string {
			return color.SprintFunc()(text)
		},
	}
	t := "{{color .PodColor .PodName}} {{color .ContainerColor .ContainerName}} {{.Message}}\n"

	tmpl, err := template.New("logs").Funcs(funs).Parse(t)
	if err != nil {
		return nil, err
	}
	return &stern.Config{
		KubeConfig:          kubeconfigPath,
		ContextName:         okteto.UrlToKubernetesContext(okteto.Context().Name),
		Namespaces:          []string{okteto.Context().Namespace},
		PodQuery:            pod,
		ContainerQuery:      container,
		InitContainers:      true,
		EphemeralContainers: true,
		TailLines:           tailLines,
		Since:               48 * time.Hour,
		LabelSelector:       labelSelector,
		FieldSelector:       fields.Everything(),
		AllNamespaces:       false,
		Follow:              true,
		ErrOut:              os.Stderr,
		Out:                 os.Stdout,
		ContainerStates:     []stern.ContainerState{"running"},
		Template:            tmpl,
	}, nil
}
