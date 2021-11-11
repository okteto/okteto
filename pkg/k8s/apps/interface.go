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
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type App interface {
	Kind() string
	ObjectMeta() metav1.ObjectMeta
	Replicas() int32
	SetReplicas(n int32)
	TemplateObjectMeta() metav1.ObjectMeta
	PodSpec() *apiv1.PodSpec

	DevClone() App

	CheckConditionErrors(dev *model.Dev) error
	GetRunningPod(ctx context.Context, c kubernetes.Interface) (*apiv1.Pod, error)

	//TODO: remove after people move to CLI >= 1.14
	RestoreOriginal() error

	Refresh(ctx context.Context, c kubernetes.Interface) error
	Watch(ctx context.Context, result chan error, c kubernetes.Interface)
	Deploy(ctx context.Context, c kubernetes.Interface) error
	Destroy(ctx context.Context, c kubernetes.Interface) error

	Divert(username string) App
}
