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
	"context"

	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type App interface {
	TypeMeta() metav1.TypeMeta
	ObjectMeta() metav1.ObjectMeta
	Replicas() int32
	TemplateObjectMeta() metav1.ObjectMeta
	PodSpec() *apiv1.PodSpec

	NewTranslation(dev *model.Dev) *Translation
	DevModeOn()
	DevModeOff(t *Translation)
	CheckConditionErrors(dev *model.Dev) error
	GetRevision() string
	GetRunningPod(ctx context.Context, c kubernetes.Interface) (*apiv1.Pod, error)
	Divert(ctx context.Context, username string, dev *model.Dev, c kubernetes.Interface) (App, error)

	RestoreOriginal() error
	SetOriginal() error

	Refresh(ctx context.Context, c kubernetes.Interface) error
	Create(ctx context.Context, c kubernetes.Interface) error
	Update(ctx context.Context, c kubernetes.Interface) error
	Destroy(ctx context.Context, dev *model.Dev, c kubernetes.Interface) error
}

// Translation represents the information for translating a deployment
type Translation struct {
	Interactive         bool               `json:"interactive"`
	Name                string             `json:"name"`
	Version             string             `json:"version"`
	App                 App                `json:"-"`
	Annotations         model.Annotations  `json:"annotations,omitempty"`
	Labels              model.Labels       `json:"labels,omitempty"`
	Tolerations         []apiv1.Toleration `json:"tolerations,omitempty"`
	Replicas            int32              `json:"replicas"`
	DeploymentStrategy  appsv1.DeploymentStrategy
	StatefulsetStrategy appsv1.StatefulSetUpdateStrategy
	Rules               []*model.TranslationRule `json:"rules"`
}
