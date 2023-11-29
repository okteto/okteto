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

package apps

import (
	"encoding/json"
	"strconv"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
)

type stateBeforeSleeping struct {
	Replicas int `json:"replicas"`
}

func getPreviousAppReplicas(app App) int32 {
	previousState := app.ObjectMeta().Annotations[model.StateBeforeSleepingAnnontation]
	if previousState != "" {
		var state stateBeforeSleeping
		if err := json.Unmarshal([]byte(previousState), &state); err != nil {
			oktetoLog.Infof("error getting previous state of '%s': %s", app.ObjectMeta().Name, err.Error())
			return 1
		}
		return int32(state.Replicas)
	}

	if rString, ok := app.ObjectMeta().Annotations[model.AppReplicasAnnotation]; ok {
		rInt, err := strconv.ParseInt(rString, 10, 32)
		if err != nil {
			oktetoLog.Infof("error parsing app replicas: %v", err)
			return 1
		}
		return int32(rInt)
	}

	return app.Replicas()
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
