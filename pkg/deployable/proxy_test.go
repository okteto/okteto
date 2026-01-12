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

package deployable

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
)

type fakeKubeConfig struct {
	config      *rest.Config
	errOnModify error
	errRead     error
}

func (f *fakeKubeConfig) Read() (*rest.Config, error) {
	if f.errRead != nil {
		return nil, f.errRead
	}
	return f.config, nil
}

func (fc *fakeKubeConfig) Modify(_ int, _, _ string) error {
	return fc.errOnModify
}

func Test_NewProxy(t *testing.T) {
	dnsErr := &net.DNSError{
		IsNotFound: true,
	}

	tests := []struct {
		expectedErr    error
		portGetter     func(string) (int, error)
		fakeKubeconfig *fakeKubeConfig
		expectedProxy  *Proxy
		name           string
	}{
		{
			name:        "err getting port, DNS not found error",
			portGetter:  func(string) (int, error) { return 0, dnsErr },
			expectedErr: dnsErr,
		},
		{
			name:        "err getting port, any error",
			portGetter:  func(string) (int, error) { return 0, assert.AnError },
			expectedErr: assert.AnError,
		},
		{
			name:       "err reading kubeconfig",
			portGetter: func(string) (int, error) { return 0, nil },
			fakeKubeconfig: &fakeKubeConfig{
				errRead: assert.AnError,
			},
			expectedErr: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewProxy(tt.fakeKubeconfig, tt.portGetter)
			require.Equal(t, tt.expectedProxy, got)
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestDecodeObject_StandardResource_JSON(t *testing.T) {
	// Create a Deployment and encode it as JSON
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(3),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "test"},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "nginx", Image: "nginx:latest"},
					},
				},
			},
		},
	}

	// Encode as JSON
	encoder := getEncoder("application/json")
	var encoded []byte
	buf := &bytesBuffer{buf: &encoded}
	err := encoder.Encode(deployment, buf)
	require.NoError(t, err)

	// Decode it back
	obj, err := decodeObject(encoded, "application/json")
	require.NoError(t, err)
	require.NotNil(t, obj)

	// Verify it's decoded as a typed Deployment, not unstructured
	decodedDeployment, ok := obj.(*appsv1.Deployment)
	require.True(t, ok, "expected *appsv1.Deployment, got %T", obj)
	assert.Equal(t, "test-deployment", decodedDeployment.Name)
	assert.Equal(t, "default", decodedDeployment.Namespace)
}

func TestDecodeObject_CRD_JSON(t *testing.T) {
	// Create a CRD (External resource) as JSON
	crdJSON := []byte(`{
		"apiVersion": "dev.okteto.com/v1",
		"kind": "External",
		"metadata": {
			"name": "test-external",
			"namespace": "default",
			"labels": {
				"app": "test"
			}
		},
		"spec": {
			"name": "test-external",
			"endpoints": [
				{
					"name": "endpoint1",
					"url": "https://example.com"
				}
			]
		}
	}`)

	// Decode it - should fallback to unstructured since External is not in the standard scheme
	obj, err := decodeObject(crdJSON, "application/json")
	require.NoError(t, err)
	require.NotNil(t, obj)

	// Verify it's decoded as unstructured
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	require.True(t, ok, "expected *unstructured.Unstructured, got %T", obj)
	assert.Equal(t, "External", unstructuredObj.GetKind())
	assert.Equal(t, "dev.okteto.com/v1", unstructuredObj.GetAPIVersion())
	assert.Equal(t, "test-external", unstructuredObj.GetName())
	assert.Equal(t, "default", unstructuredObj.GetNamespace())
}

func TestDecodeObject_StandardResource_ApplyPatchYAML(t *testing.T) {
	// Create a Deployment as YAML (server-side apply format)
	deploymentYAML := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: nginx
        image: nginx:latest
`)

	// Decode it
	obj, err := decodeObject(deploymentYAML, "application/apply-patch+yaml")
	require.NoError(t, err)
	require.NotNil(t, obj)

	// Verify it's decoded as a typed Deployment
	deployment, ok := obj.(*appsv1.Deployment)
	require.True(t, ok, "expected *appsv1.Deployment, got %T", obj)
	assert.Equal(t, "test-deployment", deployment.Name)
	assert.Equal(t, int32(3), *deployment.Spec.Replicas)
}

func TestDecodeObject_InvalidData(t *testing.T) {
	// Try to decode completely invalid data
	invalidData := []byte("this is not valid kubernetes resource data")

	obj, err := decodeObject(invalidData, "application/json")
	require.Error(t, err)
	require.Nil(t, obj)
	assert.Contains(t, err.Error(), "error decoding object")
}

func TestDecodeObject_EmptyData(t *testing.T) {
	// Try to decode empty data
	obj, err := decodeObject([]byte{}, "application/json")
	require.Error(t, err)
	require.Nil(t, obj)
}

func TestGetEncoder_DifferentContentTypes(t *testing.T) {

	tests := []struct {
		name        string
		contentType string
		expectType  string
	}{
		{
			name:        "JSON encoder",
			contentType: "application/json",
			expectType:  "json",
		},
		{
			name:        "Apply patch YAML encoder",
			contentType: "application/apply-patch+yaml",
			expectType:  "json", // YAML is handled as JSON in K8s
		},
		{
			name:        "Protobuf encoder",
			contentType: "application/vnd.kubernetes.protobuf",
			expectType:  "protobuf",
		},
		{
			name:        "Unknown content type defaults to JSON",
			contentType: "application/unknown",
			expectType:  "json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoder := getEncoder(tt.contentType)
			require.NotNil(t, encoder)

			// Verify we can encode a simple Deployment
			deployment := &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			}

			var encoded []byte
			buf := &bytesBuffer{buf: &encoded}
			err := encoder.Encode(deployment, buf)
			require.NoError(t, err)
			assert.NotEmpty(t, encoded)
		})
	}
}

// Helper type for testing encoding
type bytesBuffer struct {
	buf *[]byte
}

func (b *bytesBuffer) Write(p []byte) (n int, err error) {
	*b.buf = append(*b.buf, p...)
	return len(p), nil
}

func int32Ptr(i int32) *int32 {
	return &i
}

// TestPartialDeploymentServerSideApply tests that partial objects (minimal YAML)
// can be decoded, modified, and encoded without errors
func TestPartialDeploymentServerSideApply(t *testing.T) {
	// Simulate a very minimal server-side apply payload
	// This is the kind of partial object we might receive in a PATCH request
	partialDeploymentYAML := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
  namespace: default
`)

	// Step 1: Decode
	obj, err := decodeObject(partialDeploymentYAML, "application/apply-patch+yaml")
	require.NoError(t, err)
	require.NotNil(t, obj)

	deployment, ok := obj.(*appsv1.Deployment)
	require.True(t, ok, "expected *appsv1.Deployment, got %T", obj)
	assert.Equal(t, "test-deployment", deployment.Name)
	assert.Equal(t, "default", deployment.Namespace)

	// Step 2: Modify using Translator
	translator := newTranslator("test-deployer", nil)
	err = translator.Modify(obj)
	require.NoError(t, err)

	// Step 3: Verify labels and annotations were added correctly
	// Check metadata labels
	require.NotNil(t, deployment.Labels)
	assert.Equal(t, "test-deployer", deployment.Labels["dev.okteto.com/deployed-by"],
		"deployed-by label should be set in metadata")

	// Check metadata annotations
	require.NotNil(t, deployment.Annotations)
	assert.Equal(t, "true", deployment.Annotations["dev.okteto.com/sample"],
		"sample annotation should be set in metadata")

	// Check template labels
	require.NotNil(t, deployment.Spec.Template.Labels)
	assert.Equal(t, "test-deployer", deployment.Spec.Template.Labels["dev.okteto.com/deployed-by"],
		"deployed-by label should be set in template")

	// Step 4: Encode back
	encoder := getEncoder("application/apply-patch+yaml")
	var encoded []byte
	buf := &bytesBuffer{buf: &encoded}
	err = encoder.Encode(deployment, buf)
	require.NoError(t, err)
	assert.NotEmpty(t, encoded)
}
