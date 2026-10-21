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

package noop

import (
	"context"

	istioNetworkingV1beta1 "istio.io/api/networking/v1beta1"
	apiv1 "k8s.io/api/core/v1"
)

// Driver struct for noop divert driver. It is an empty implementation for flows
// where divert is not defined in the manifest
type Driver struct{}

func (*Driver) Deploy(_ context.Context) error {
	return nil
}

// Destroy implements from the interface diver.Driver
// nolint:unparam
func (*Driver) Destroy(_ context.Context) error {
	return nil
}

func (*Driver) UpdatePod(pod apiv1.PodSpec) apiv1.PodSpec {
	return pod
}

func (*Driver) UpdateVirtualService(_ *istioNetworkingV1beta1.VirtualService) {}
