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

package insights

import (
	"context"
	"encoding/json"

	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/repository"
)

const (
	// deployInsightType represents the type of the deploy event
	deployInsightType = "deploy"

	// deploySchemaVersion represents the schema version of the deploy event
	// This version should be updated if the structure of the event changes
	deploySchemaVersion = "1.0"
)

// deployEventJSON represents the JSON structure of a deploy event
type deployEventJSON struct {
	DevenvName    string      `json:"devenv_name"`
	Repository    string      `json:"repository"`
	Namespace     string      `json:"namespace"`
	SchemaVersion string      `json:"schema_version"`
	Phase         []phaseJSON `json:"phases"`
	Success       bool        `json:"success"`
}

// phaseJSON represents the JSON structure of a phase in a deploy event
type phaseJSON struct {
	Name     string  `json:"name"`
	Duration float64 `json:"duration"`
}

// TrackDeploy tracks an image build event
func (ip *Publisher) TrackDeploy(ctx context.Context, name, namespace string, success bool) {
	k8sClient, _, err := ip.k8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		ip.ioCtrl.Logger().Infof("could not get k8s client: %s", err)
		return
	}
	cfgName := pipeline.TranslatePipelineName(name)
	cmap, err := configmaps.Get(ctx, cfgName, namespace, k8sClient)
	if err != nil {
		ip.ioCtrl.Logger().Infof("could not get pipeline configmap: %s", err)
		return
	}

	val, ok := cmap.Data[pipeline.PhasesField]
	// If there is no phases, we don't track the event
	if !ok {
		ip.ioCtrl.Logger().Infof("no phases found in pipeline configmap. Skipping event tracking")
		return
	}

	var phases []phaseJSON
	if err := json.Unmarshal([]byte(val), &phases); err != nil {
		ip.ioCtrl.Logger().Infof("could not unmarshal phases from cmap: %s", err)
		return
	}

	repo := cmap.Data["repository"]

	deployEvent := &deployEventJSON{
		DevenvName:    name,
		Repository:    repository.NewRepository(repo).GetAnonymizedRepo(),
		Namespace:     namespace,
		Phase:         phases,
		Success:       success,
		SchemaVersion: deploySchemaVersion,
	}

	eventJSON, err := json.Marshal(deployEvent)
	if err != nil {
		ip.ioCtrl.Logger().Infof("could not marshal deploy event: %s", err)
	}

	ip.trackEvent(ctx, namespace, deployInsightType, string(eventJSON))
}
