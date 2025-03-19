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

package endpoints

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Getter represents the endpoints getter
type Getter struct {
	client kubernetes.Interface
}

// NewGetter returns a new endpoints getter
func NewGetter(client kubernetes.Interface) *Getter {
	return &Getter{client: client}
}

func (g Getter) GetByName(ctx context.Context, name, namespace string) (*corev1.Endpoints, error) {
	e, err := g.client.CoreV1().Endpoints(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting kubernetes endpoint: %s", err)
	}
	return e, nil
}
