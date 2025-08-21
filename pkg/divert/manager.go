// Copyright 2025 The Okteto Authors
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

package divert

import (
	"context"
	"time"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/divert/k8s"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DivertManager is the manager for Divert resources in Kubernetes.
type DivertManager struct {
	Client k8s.DivertV1Interface
}

func NewDivertManager(client k8s.DivertV1Interface) *DivertManager {
	return &DivertManager{
		Client: client,
	}
}

// CreateOrUpdate creates or updates a Divert resource in Kubernetes.
// If the resource already exists, it updates it; otherwise, it creates a new one.
// It returns an error if the operation fails.
func (dm *DivertManager) CreateOrUpdate(ctx context.Context, d *k8s.Divert) error {
	old, err := dm.Client.Diverts(d.Namespace).Get(ctx, d.Name, metav1.GetOptions{})
	if err != nil {
		if k8sErrors.IsNotFound(err) {
			return dm.create(ctx, d)
		}

		return err
	}

	old.Spec = d.Spec
	return dm.update(ctx, old)
}

func (dm *DivertManager) create(ctx context.Context, d *k8s.Divert) error {
	if d.Annotations == nil {
		d.Annotations = make(map[string]string)
	}
	d.Annotations[constants.LastUpdatedAnnotation] = time.Now().UTC().Format(constants.TimeFormat)
	_, err := dm.Client.Diverts(d.Namespace).Create(ctx, d, metav1.CreateOptions{})
	return err
}

func (dm *DivertManager) update(ctx context.Context, d *k8s.Divert) error {
	if d.Annotations == nil {
		d.Annotations = make(map[string]string)
	}
	d.Annotations[constants.LastUpdatedAnnotation] = time.Now().UTC().Format(constants.TimeFormat)
	_, err := dm.Client.Diverts(d.Namespace).Update(ctx, d)
	return err
}
