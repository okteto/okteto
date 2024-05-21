// Copyright 2024 The Okteto Authors
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
	"context"
	"fmt"

	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	"github.com/okteto/okteto/pkg/model"
	v1 "k8s.io/api/apps/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
)

// IsDeployed checks if a pipeline has been
func IsDeployed(ctx context.Context, name, namespace string, c kubernetes.Interface) bool {
	cmap, err := configmaps.Get(ctx, TranslatePipelineName(name), namespace, c)
	if (err != nil && k8sErrors.IsNotFound(err)) || cmap == nil {
		return false
	}
	return cmap.Data[statusField] != ErrorStatus
}

// ListDeployments list all the deployments created by the pipeline
func ListDeployments(ctx context.Context, name, ns string, c kubernetes.Interface) ([]v1.Deployment, error) {
	labels := fmt.Sprintf("%s=%s", model.DeployedByLabel, format.ResourceK8sMetaString(name))
	dList, err := deployments.List(ctx, ns, labels, c)
	if err != nil {
		return nil, err
	}
	return dList, nil
}

// ListStatefulsets list all the sfs created by the pipeline
func ListStatefulsets(ctx context.Context, name, ns string, c kubernetes.Interface) ([]v1.StatefulSet, error) {
	labels := fmt.Sprintf("%s=%s", model.DeployedByLabel, format.ResourceK8sMetaString(name))
	sfsList, err := statefulsets.List(ctx, ns, labels, c)
	if err != nil {
		return nil, err
	}
	return sfsList, nil
}

// HasDeployedSomething checks if the pipeline has deployed any deployment/statefulset/job
func HasDeployedSomething(ctx context.Context, name, ns string, c kubernetes.Interface) (bool, error) {
	labels := fmt.Sprintf("%s=%s", model.DeployedByLabel, format.ResourceK8sMetaString(name))
	dList, err := deployments.List(ctx, ns, labels, c)
	if err != nil {
		return false, err
	}
	if len(dList) > 0 {
		return true, nil
	}

	sfsList, err := statefulsets.List(ctx, ns, labels, c)
	if err != nil {
		return false, err
	}
	if len(sfsList) > 0 {
		return true, nil
	}

	return false, nil
}
