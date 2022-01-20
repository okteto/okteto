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
	"encoding/base64"

	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	nameField   = "name"
	statusField = "status"
	outputField = "output"

	// ProgressingStatus indicates that an app is being deployed
	ProgressingStatus = "progressing"
	// DeployedStatus indicates that an app is deployed
	DeployedStatus = "deployed"
	// ErrorStatus indicates that an app has errors
	ErrorStatus = "error"
	// DestroyingStatus indicates that an app is being destroyed
	DestroyingStatus = "destroying"
)

// TranslateConfigMap translates the app into a configMap
func TranslateConfigMap(name, status, output string) *apiv1.ConfigMap {
	return &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				model.GitDeployLabel: "true",
			},
		},
		Data: map[string]string{
			nameField:   name,
			statusField: status,
			outputField: base64.StdEncoding.EncodeToString([]byte(output)),
		},
	}
}

// SetStatus sets the status and output of a config map
func SetStatus(cfg *apiv1.ConfigMap, status, output string) *apiv1.ConfigMap {
	cfg.Data[statusField] = status
	cfg.Data[outputField] = base64.StdEncoding.EncodeToString([]byte(output))
	return cfg
}
