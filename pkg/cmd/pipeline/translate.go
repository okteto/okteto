// Copyright 2021 The Okteto Authors
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
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	nameField       = "name"
	statusField     = "status"
	outputField     = "output"
	repoField       = "repository"
	branchField     = "branch"
	filenameField   = "filename"
	yamlField       = "yaml"
	iconField       = "icon"
	actionLockField = "actionLock"
	actionNameField = "actionName"

	actionDefaultName = "cli"

	// ProgressingStatus indicates that an app is being deployed
	ProgressingStatus = "progressing"
	// DeployedStatus indicates that an app is deployed
	DeployedStatus = "deployed"
	// ErrorStatus indicates that an app has errors
	ErrorStatus = "error"
	// DestroyingStatus indicates that an app is being destroyed
	DestroyingStatus = "destroying"

	// maxLogOutput is the maximum size that we allow to allocate for logs.
	// Specifically 800kb. The limit on configmaps is 1Mb but we want to leave some
	// room for the other data stored in there.
	// Note that the is the limit after encoding the logs to base64 which is how
	// the logs are stored in the configmap.
	maxLogOutput = 800 << (10 * 1)
)

// maxLogOutputRaw is the maximum size we allow to allocate for logs before
// being encoded to base64
// See: https://stackoverflow.com/a/4715480/1100238
var maxLogOutputRaw = int(math.Floor(float64(maxLogOutput)*3) / 4)

// CfgData represents the data to be include in a configmap
type CfgData struct {
	Name       string
	Namespace  string
	Status     string
	Output     string
	Repository string
	Branch     string
	Filename   string
	Manifest   []byte
	Icon       string
}

// TranslateConfigMapAndDeploy translates the app into a configMap.
// Name param is the pipeline sanitized name
func TranslateConfigMapAndDeploy(ctx context.Context, data *CfgData, c kubernetes.Interface) (*apiv1.ConfigMap, error) {
	cmap, err := configmaps.Get(ctx, TranslatePipelineName(data.Name), data.Namespace, c)
	if err != nil {
		if !oktetoErrors.IsNotFound(err) {
			return nil, err
		}
		cmap = translateConfigMapSandBox(data)
		err := configmaps.Create(ctx, cmap, cmap.Namespace, c)
		if err != nil {
			if k8sErrors.IsAlreadyExists(err) {
				return nil, errors.New("There is a pipeline operation already running")
			}
			return nil, err
		}
	}

	if err := updateCmap(cmap, data); err != nil {
		return nil, err
	}
	if err := configmaps.Deploy(ctx, cmap, cmap.Namespace, c); err != nil {
		if k8sErrors.IsConflict(err) {
			return nil, errors.New("There is a pipeline operation already running")
		}
		return nil, err
	}
	return cmap, nil
}

// SetOutput sets the output of a config map
func SetOutput(cfg *apiv1.ConfigMap, output string) *apiv1.ConfigMap {
	cfg.Data[outputField] = base64.StdEncoding.EncodeToString([]byte(output))
	return cfg
}

// UpdateConfigMap updates the configmaps fields
func UpdateConfigMap(ctx context.Context, cmap *apiv1.ConfigMap, data *CfgData, c kubernetes.Interface) error {
	cmap, err := configmaps.Get(ctx, cmap.Name, cmap.Namespace, c)
	if err != nil {
		return err
	}
	if err := updateCmap(cmap, data); err != nil {
		return err
	}
	return configmaps.Deploy(ctx, cmap, cmap.Namespace, c)
}

// TranslatePipelineName translate the name into the configmap name
func TranslatePipelineName(name string) string {
	return fmt.Sprintf("okteto-git-%s", format.ResourceK8sMetaString(name))
}

func translateOutput(output *bytes.Buffer) []byte {
	// If the output is larger than the currentMaxLimit for the logs trim it.
	// We can't really truncate the buffer since we would end up with an invalid json
	// line for the last line, so we pick lines from the end while the line fits
	var data []byte
	if output.Len() > maxLogOutputRaw {
		scanner := bufio.NewScanner(output)
		linesInReverse := []string{}
		for scanner.Scan() {
			linesInReverse = append([]string{scanner.Text()}, linesInReverse...)
		}
		var cappedOutput string
		var head string
		for _, l := range linesInReverse {
			head = l + "\n"
			if len(cappedOutput)+len(head) > maxLogOutputRaw {
				break
			}
			cappedOutput = head + cappedOutput
		}
		cappedOutput = strings.TrimPrefix(cappedOutput, head)
		data = []byte(cappedOutput)
	} else {
		data = output.Bytes()
	}
	return data
}

// translateConfigMapSandBox creates a configmap adding data from a config data
func translateConfigMapSandBox(data *CfgData) *apiv1.ConfigMap {
	// if repository is empty, force empty branch
	if data.Repository == "" {
		data.Branch = ""
	}
	cmap := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: data.Namespace,
			Name:      TranslatePipelineName(format.ResourceK8sMetaString(data.Name)),
			Annotations: map[string]string{
				model.LastUpdatedAnnotation: time.Now().UTC().Format(model.TimeFormat),
			},
			Labels: map[string]string{
				model.GitDeployLabel: "true",
			},
		},
		Data: map[string]string{
			nameField:     data.Name,
			statusField:   data.Status,
			repoField:     data.Repository,
			branchField:   data.Branch,
			filenameField: "",
			yamlField:     base64.StdEncoding.EncodeToString(data.Manifest),
			iconField:     data.Icon,
		},
	}
	if data.Repository != "" {
		cmap.Data[filenameField] = data.Filename
	}

	output := oktetoLog.GetOutputBuffer()
	outputData := translateOutput(output)
	cmap.Data[outputField] = base64.StdEncoding.EncodeToString([]byte(outputData))
	return cmap
}

func updateCmap(cmap *apiv1.ConfigMap, data *CfgData) error {
	if cmap.Annotations == nil {
		cmap.Annotations = map[string]string{}
	}
	cmap.Annotations[model.LastUpdatedAnnotation] = time.Now().UTC().Format(model.TimeFormat)

	actionName := os.Getenv(model.OktetoActionNameEnvVar)
	if actionName == "" {
		actionName = actionDefaultName
	}
	if val, ok := cmap.Data[actionLockField]; ok && val != actionName {
		return errors.New("There is a pipeline operation already running")
	}
	cmap.ObjectMeta.Labels[model.GitDeployLabel] = "true"
	cmap.Data[nameField] = data.Name
	cmap.Data[statusField] = data.Status
	cmap.Data[yamlField] = base64.StdEncoding.EncodeToString(data.Manifest)
	cmap.Data[iconField] = data.Icon
	cmap.Data[actionNameField] = actionName
	if data.Repository != "" {
		// the filename at the cfgmap is used by the installer to re-deploy the app from the ui
		// this parameter is just saved if a repository is being detected
		// when repository is empty - the filename should not be saved and redeploys should be done from cli
		cmap.Data[filenameField] = data.Filename
		cmap.Data[repoField] = data.Repository
	}

	// if repository is empty, force empty branch
	if data.Repository == "" {
		data.Branch = ""
	}

	if data.Branch != "" {
		cmap.Data[branchField] = data.Branch
	}

	output := oktetoLog.GetOutputBuffer()
	outputData := translateOutput(output)
	cmap.Data[outputField] = base64.StdEncoding.EncodeToString([]byte(outputData))
	return nil
}

// AddDevAnnotations add deploy labels to the deployments/sfs
func AddDevAnnotations(ctx context.Context, manifest *model.Manifest, c kubernetes.Interface) {
	repo := os.Getenv(model.GithubRepositoryEnvVar)
	for devName, dev := range manifest.Dev {
		if dev.Autocreate {
			continue
		}
		app, err := apps.Get(ctx, dev, manifest.Namespace, c)
		if err != nil {
			oktetoLog.Infof("could not add %s dev annotations due to: %s", devName, err.Error())
			continue
		}
		if repo != "" {
			app.ObjectMeta().Annotations[model.OktetoRepositoryAnnotation] = repo
		}
		app.ObjectMeta().Annotations[model.OktetoDevNameAnnotation] = devName
		if err := app.PatchAnnotations(ctx, c); err != nil {
			oktetoLog.Infof("could not add %s dev annotations due to: %s", devName, err.Error())
		}
	}
}
