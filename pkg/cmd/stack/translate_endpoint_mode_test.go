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

package stack

import (
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
)

func TestTranslateServiceWithEndpointModeVIP(t *testing.T) {
	stack := &model.Stack{
		Name: "test-stack",
		Services: map[string]*model.Service{
			"web": {
				Image:        "nginx:latest",
				EndpointMode: "vip",
				Ports: []model.Port{
					{ContainerPort: 80, HostPort: 8080},
				},
			},
		},
	}

	result := translateService("web", stack)

	assert.Equal(t, "web", result.Name)
	assert.Equal(t, apiv1.ServiceTypeClusterIP, result.Spec.Type)
	// ClusterIP should not be set to "None" for vip mode (default behavior)
	assert.Empty(t, result.Spec.ClusterIP)
	assert.Len(t, result.Spec.Ports, 2) // Should have both 80 and 8080 ports
}

func TestTranslateServiceWithEndpointModeDNSRR(t *testing.T) {
	stack := &model.Stack{
		Name: "test-stack",
		Services: map[string]*model.Service{
			"web": {
				Image:        "nginx:latest",
				EndpointMode: "dnsrr",
				Ports: []model.Port{
					{ContainerPort: 80, HostPort: 8080},
				},
			},
		},
	}

	result := translateService("web", stack)

	assert.Equal(t, "web", result.Name)
	assert.Equal(t, apiv1.ServiceTypeClusterIP, result.Spec.Type)
	// ClusterIP should be set to "None" for dnsrr mode (headless service)
	assert.Equal(t, "None", result.Spec.ClusterIP)
	assert.Len(t, result.Spec.Ports, 2) // Should have both 80 and 8080 ports
}

func TestTranslateServiceWithEndpointModeDefault(t *testing.T) {
	stack := &model.Stack{
		Name: "test-stack",
		Services: map[string]*model.Service{
			"web": {
				Image:        "nginx:latest",
				EndpointMode: "", // Empty means no endpoint_mode specified
				Ports: []model.Port{
					{ContainerPort: 80},
				},
			},
		},
	}

	result := translateService("web", stack)

	assert.Equal(t, "web", result.Name)
	assert.Equal(t, apiv1.ServiceTypeClusterIP, result.Spec.Type)
	// ClusterIP should not be set when endpoint_mode is not specified (no modifications)
	assert.Empty(t, result.Spec.ClusterIP)
}

func TestTranslateServiceLabelsAndAnnotations(t *testing.T) {
	stack := &model.Stack{
		Name: "test-stack",
		Services: map[string]*model.Service{
			"web": {
				Image:        "nginx:latest",
				EndpointMode: "dnsrr",
				Labels: map[string]string{
					"app": "web",
				},
				Annotations: map[string]string{
					"service.annotation": "value",
				},
				Ports: []model.Port{
					{ContainerPort: 80},
				},
			},
		},
	}

	result := translateService("web", stack)

	// Verify that setting endpoint_mode doesn't affect other service properties
	assert.Equal(t, "None", result.Spec.ClusterIP)
	assert.Contains(t, result.Labels, model.StackNameLabel)
	assert.Contains(t, result.Labels, model.StackServiceNameLabel)
	assert.Contains(t, result.Labels, "app")
}

func TestTranslateServiceMultipleServices(t *testing.T) {
	stack := &model.Stack{
		Name: "test-stack",
		Services: map[string]*model.Service{
			"frontend": {
				Image:        "nginx:latest",
				EndpointMode: "vip",
				Ports:        []model.Port{{ContainerPort: 80}},
			},
			"backend": {
				Image:        "api:latest",
				EndpointMode: "dnsrr",
				Ports:        []model.Port{{ContainerPort: 3000}},
			},
		},
	}

	frontendResult := translateService("frontend", stack)
	backendResult := translateService("backend", stack)

	// Frontend should have default ClusterIP behavior (vip)
	assert.Empty(t, frontendResult.Spec.ClusterIP)
	
	// Backend should be headless service (dnsrr)
	assert.Equal(t, "None", backendResult.Spec.ClusterIP)
}