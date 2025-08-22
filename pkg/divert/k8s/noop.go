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

package k8s

import (
	"context"

	"github.com/okteto/okteto/pkg/log/io"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// NoopV1DivertClient is a no-operation implementation of the DivertV1Interface
type NoopV1DivertClient struct {
	client *NoopDivertClient
}

// NoopDivertClient implements DivertInterface but does not perform any operations in the cluster
type NoopDivertClient struct {
	IOCtrl *io.Controller
}

func (n *NoopV1DivertClient) Diverts(_ string) DivertInterface {
	return n.client
}

func (n *NoopDivertClient) Create(_ context.Context, _ *Divert, _ metav1.CreateOptions) (*Divert, error) {
	n.IOCtrl.Logger().Debugf("NoopDivertClient: Create called, but no operation performed")
	return nil, nil
}

func (n *NoopDivertClient) Get(_ context.Context, _ string, _ metav1.GetOptions) (*Divert, error) {
	n.IOCtrl.Logger().Debugf("NoopDivertClient: Get called, but no operation performed")
	return nil, k8sErrors.NewNotFound(schema.GroupResource{Group: GroupName, Resource: DivertResource}, "not found")
}

func (n *NoopDivertClient) Update(_ context.Context, _ *Divert) (*Divert, error) {
	n.IOCtrl.Logger().Debugf("NoopDivertClient: Update called, but no operation performed")
	return nil, k8sErrors.NewNotFound(schema.GroupResource{Group: GroupName, Resource: DivertResource}, "not found")
}

func (n *NoopDivertClient) Delete(_ context.Context, _ string, _ metav1.DeleteOptions) error {
	n.IOCtrl.Logger().Debugf("NoopDivertClient: Delete called, but no operation performed")
	return nil
}

func (n *NoopDivertClient) List(_ context.Context, _ metav1.ListOptions) (*DivertList, error) {
	n.IOCtrl.Logger().Debugf("NoopDivertClient: List called, but no operation performed")
	return &DivertList{Items: make([]Divert, 0)}, nil
}
