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

	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/types"
	giturls "github.com/whilp/git-urls"
	apiv1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
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
	variablesField  = "variables"

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
	Variables  []string
}

// GetConfigmapVariablesEncoded returns Data["variables"] content from Configmap
func GetConfigmapVariablesEncoded(ctx context.Context, name, namespace string, c kubernetes.Interface) (string, error) {
	cmap, err := configmaps.Get(ctx, TranslatePipelineName(name), namespace, c)
	if err != nil {
		if !oktetoErrors.IsNotFound(err) {
			return "", err
		}
		// if err Not Found, return empty variables but no error
		return "", nil
	}

	return cmap.Data[variablesField], nil
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

// UpdateEnvs updates the configmap adding the envs as data fields
func UpdateEnvs(ctx context.Context, name, namespace string, envs []string, c kubernetes.Interface) error {
	cmap, err := configmaps.Get(ctx, TranslatePipelineName(name), namespace, c)
	if err != nil {
		return err
	}

	if cmap != nil {
		envsToSet := make(map[string]string, len(envs))
		for _, env := range envs {
			result := strings.Split(env, "=")
			if len(result) != 2 {
				return fmt.Errorf("invalid env format: '%s'", env)
			}

			envsToSet[result[0]] = result[1]
		}

		if len(envsToSet) > 0 {
			encondedEnvs, err := json.Marshal(envsToSet)
			if err != nil {
				return err
			}
			cmap.Data[constants.OktetoDependencyEnvsKey] = base64.StdEncoding.EncodeToString(encondedEnvs)
			return configmaps.Deploy(ctx, cmap, cmap.Namespace, c)
		}

	}
	return nil
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
		var linesInReverse []string
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
				constants.LastUpdatedAnnotation: time.Now().UTC().Format(constants.TimeFormat),
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

	// only include field when variables exist
	if len(data.Variables) > 0 {
		cmap.Data[variablesField] = translateVariables(data.Variables)
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
	cmap.Annotations[constants.LastUpdatedAnnotation] = time.Now().UTC().Format(constants.TimeFormat)

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

	// only update field when variables exist
	if len(data.Variables) > 0 {
		cmap.Data[variablesField] = translateVariables(data.Variables)
	} else {
		// if data.Variables is empty, update cmap by removing the field
		delete(cmap.Data, variablesField)
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
			app.ObjectMeta().Annotations[model.OktetoRepositoryAnnotation] = removeSensitiveDataFromGitURL(repo)
		}
		app.ObjectMeta().Annotations[model.OktetoDevNameAnnotation] = devName
		if err := app.PatchAnnotations(ctx, c); err != nil {
			oktetoLog.Infof("could not add %s dev annotations due to: %s", devName, err.Error())
		}
	}
}

func removeSensitiveDataFromGitURL(gitURL string) string {
	if gitURL == "" {
		return gitURL
	}

	parsedRepo, err := giturls.Parse(gitURL)
	if err != nil {
		return ""
	}

	if parsedRepo.User.Username() != "" {
		parsedRepo.User = nil
	}
	return parsedRepo.String()
}

func translateVariables(variables []string) string {
	var v []types.DeployVariable
	for _, item := range variables {
		splitV := strings.SplitN(item, "=", 2)
		if len(splitV) != 2 {
			continue
		}
		v = append(v, types.DeployVariable{
			Name:  splitV[0],
			Value: splitV[1],
		})
	}

	if len(v) > 0 {
		encodedVars, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return base64.StdEncoding.EncodeToString(encodedVars)
	}

	return ""
}
