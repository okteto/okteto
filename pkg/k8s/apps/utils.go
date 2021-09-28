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

package apps

import (
	"encoding/json"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
)

type stateBeforeSleeping struct {
	Replicas int
}

func setTranslationAsAnnotation(tr *Translation) error {
	translationBytes, err := json.Marshal(tr)
	if err != nil {
		return err
	}
	tr.App.TemplateObjectMeta().Annotations[model.TranslationAnnotation] = string(translationBytes)
	return nil
}

func getTranslationFromAnnotation(annotations map[string]string) (*Translation, error) {
	tr := &Translation{}
	err := json.Unmarshal([]byte(annotations[model.TranslationAnnotation]), &tr)
	if err != nil {
		return nil, err
	}
	return tr, nil
}

func getPreviousAppReplicas(app App) int32 {
	replicas := app.Replicas()
	previousState := app.ObjectMeta().Annotations[model.StateBeforeSleepingAnnontation]
	if previousState == "" {
		return replicas
	}
	var state stateBeforeSleeping
	if err := json.Unmarshal([]byte(previousState), &state); err != nil {
		log.Infof("error getting previous state of '%s': %s", app.ObjectMeta().Name, err.Error())
		return 1
	}
	return int32(state.Replicas)
}

func deleteUserAnnotations(annotations map[string]string, tr *Translation) error {
	if tr.Annotations == nil {
		return nil
	}
	for key := range tr.Annotations {
		delete(annotations, key)
	}
	return nil
}

func GetDevContainer(spec *apiv1.PodSpec, containerName string) *apiv1.Container {
	if containerName == "" {
		return &spec.Containers[0]
	}

	for i := range spec.Containers {
		if spec.Containers[i].Name == containerName {
			return &spec.Containers[i]
		}
	}

	return nil
}
