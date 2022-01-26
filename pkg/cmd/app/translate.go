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

package app

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"math"
	"strings"

	"github.com/okteto/okteto/pkg/k8s/configmaps"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	nameField     = "name"
	statusField   = "status"
	outputField   = "output"
	repoField     = "repository"
	branchField   = "branch"
	filenameField = "filename"
	yamlField     = "yaml"
	iconField     = "icon"

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

//CfgData represents the data to be include in a configmap
type CfgData struct {
	Name       string
	Status     string
	Output     string
	Repository string
	Branch     string
	Filename   string
	Manifest   []byte
	Icon       string
}

// TranslateConfigMap translates the app into a configMap
func TranslateConfigMap(name string, data *CfgData) *apiv1.ConfigMap {
	cfmap := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: TranslateAppName(name),
			Labels: map[string]string{
				model.GitDeployLabel: "true",
			},
		},
		Data: map[string]string{
			nameField:     name,
			statusField:   data.Status,
			repoField:     data.Repository,
			branchField:   data.Branch,
			filenameField: data.Filename,
			yamlField:     base64.StdEncoding.EncodeToString(data.Manifest),
			iconField:     data.Icon,
		},
	}
	if data.Repository != "" {
		cfmap.Data[repoField] = data.Repository
	}

	if data.Branch != "" {
		cfmap.Data[branchField] = data.Branch
	}

	output := oktetoLog.GetOutputBuffer()

	outputData := translateOutput(output)

	cfmap = SetOutput(cfmap, string(outputData))
	return cfmap
}

// SetStatus sets the status and output of a config map
func SetStatus(cfg *apiv1.ConfigMap, status, output string) *apiv1.ConfigMap {
	cfg.Data[statusField] = status
	cfg.Data[outputField] = base64.StdEncoding.EncodeToString([]byte(output))
	return cfg
}

// SetOutput sets the output of a config map
func SetOutput(cfg *apiv1.ConfigMap, output string) *apiv1.ConfigMap {
	cfg.Data[outputField] = base64.StdEncoding.EncodeToString([]byte(output))
	return cfg
}

// UpdateOutput updates the configmap output with the logs
func UpdateOutput(ctx context.Context, name, namespace string, output *bytes.Buffer, c kubernetes.Interface) error {
	cmap, err := configmaps.Get(ctx, name, namespace, c)
	if err != nil {
		return err
	}

	data := translateOutput(output)

	SetOutput(cmap, string(data))

	return configmaps.Deploy(ctx, cmap, namespace, c)
}

//TranslateAppName translate the name into the pipeline name
func TranslateAppName(name string) string {
	return fmt.Sprintf("okteto-git-%s", name)
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
