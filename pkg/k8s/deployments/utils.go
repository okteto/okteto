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

package deployments

import (
	"encoding/json"

	"github.com/okteto/okteto/pkg/k8s/annotations"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type stateBeforeSleeping struct {
	Replicas int
}

func setTranslationAsAnnotation(o metav1.Object, tr *model.Translation) error {
	translationBytes, err := json.Marshal(tr)
	if err != nil {
		return err
	}
	annotations.Set(o, model.TranslationAnnotation, string(translationBytes))
	return nil
}

func getTranslationFromAnnotation(annotations map[string]string) (model.Translation, error) {
	tr := model.Translation{}
	err := json.Unmarshal([]byte(annotations[model.TranslationAnnotation]), &tr)
	if err != nil {
		return model.Translation{}, err
	}
	return tr, nil
}

func getPreviousDeploymentReplicas(d *appsv1.Deployment) int32 {
	replicas := *d.Spec.Replicas
	previousState, ok := d.Annotations[model.StateBeforeSleepingAnnontation]
	if !ok {
		return replicas
	}
	var state stateBeforeSleeping
	if err := json.Unmarshal([]byte(previousState), &state); err != nil {
		log.Infof("error getting previous state of deployment '%s': %s", d.Name, err.Error())
		return 1
	}
	return int32(state.Replicas)
}
