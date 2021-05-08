// Copyright 2020 The Okteto Authors
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

package namespaces

import (
	"context"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	okLabels "github.com/okteto/okteto/pkg/k8s/labels"
	"k8s.io/client-go/kubernetes"
)

const (
	// OktetoNotAllowedLabel tells Okteto to not allow operations on the namespace
	OktetoNotAllowedLabel = "dev.okteto.com/not-allowed"
)

// IsOktetoNamespace checks if this is a namespace created by okteto
func IsOktetoNamespace(ns *apiv1.Namespace) bool {
	return ns.Labels[okLabels.DevLabel] == "true"
}

// IsOktetoAllowed checks if Okteto operationos are allowed in this namespace
func IsOktetoAllowed(ns *apiv1.Namespace) bool {
	if _, ok := ns.Labels[OktetoNotAllowedLabel]; ok {
		return false
	}

	return true
}

// Get returns the namespace object of ns
func Get(ctx context.Context, ns string, c *kubernetes.Clientset) (*apiv1.Namespace, error) {
	n, err := c.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return n, nil
}
