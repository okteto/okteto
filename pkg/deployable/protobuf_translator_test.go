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

package deployable

import (
	"bytes"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/protobuf"
	"k8s.io/kubectl/pkg/scheme"
)

// fake object that does not support metadata.
type noMetaObject struct{}

func (n *noMetaObject) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (n *noMetaObject) DeepCopyObject() runtime.Object {
	return n
}

func TestProtobufTranslator_Translate_Success(t *testing.T) {
	// Create a sample Pod with some existing labels.
	pod := &apiv1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
			Labels: map[string]string{
				"existing": "value",
			},
		},
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{
				{Name: "container1", Image: "nginx"},
			},
		},
	}

	// Create a protobuf serializer (using the same scheme as the translator).
	pbSerializer := protobuf.NewSerializer(scheme.Scheme, scheme.Scheme)
	var buf bytes.Buffer
	err := pbSerializer.Encode(pod, &buf)
	require.NoError(t, err, "failed to encode pod to protobuf")
	inputBytes := buf.Bytes()

	translatorName := "test-deployer"
	translator := NewProtobufTranslator(inputBytes, translatorName)
	outputBytes, err := translator.Translate()
	require.NoError(t, err, "Translate returned an error")
	require.NotNil(t, outputBytes, "expected non-nil output bytes")

	decodedObj, _, err := pbSerializer.Decode(outputBytes, nil, nil)
	require.NoError(t, err, "failed to decode output bytes")

	podOut, ok := decodedObj.(*apiv1.Pod)
	require.True(t, ok, "decoded object is not a Pod")

	labels := podOut.GetLabels()
	require.Equal(t, translatorName, labels[model.DeployedByLabel], "expected deployed-by label to be set")

	annotations := podOut.GetAnnotations()
	require.NotNil(t, annotations, "annotations should not be nil after translation")
	assert.Equal(t, "true", annotations[model.OktetoSampleAnnotation], "expected okteto sample annotation to be set")
}

func TestProtobufTranslator_InvalidInput(t *testing.T) {
	invalidBytes := []byte("this is not valid protobuf data")
	translator := NewProtobufTranslator(invalidBytes, "test-deployer")
	outputBytes, err := translator.Translate()
	assert.Error(t, err, "Translate should not return an error for invalid input")
	assert.Nil(t, outputBytes, "expected output bytes to be nil for invalid input")
}

func TestProtobufTranslator_translateMetadata_NoMetadata(t *testing.T) {
	translator := NewProtobufTranslator(nil, "test-deployer")
	obj := &noMetaObject{}
	err := translator.translateMetadata(obj)
	assert.Error(t, err, "expected error when object has no metadata")
}
