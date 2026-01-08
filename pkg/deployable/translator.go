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
	"fmt"

	"github.com/okteto/okteto/pkg/divert"
	"github.com/okteto/okteto/pkg/k8s/labels"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/runtime/serializer/protobuf"
	"k8s.io/kubectl/pkg/scheme"
)

// Translator uses UniversalDeserializer to decode any format (JSON, YAML, Protobuf)
// into runtime.Object, modifies it, and re-encodes using the appropriate encoder
type Translator struct {
	name         string
	divertDriver divert.Driver
	decoder      runtime.Decoder
}

func newTranslator(name string, divertDriver divert.Driver) *Translator {
	return &Translator{
		name:         name,
		divertDriver: divertDriver,
		decoder:      scheme.Codecs.UniversalDeserializer(),
	}
}

// Translate decodes the input bytes (any format), modifies the object, and re-encodes
func (t *Translator) Translate(b []byte, contentType string) ([]byte, error) {
	// 1. Decode using UniversalDeserializer (handles JSON, YAML, Protobuf automatically)
	obj, _, err := t.decoder.Decode(b, nil, nil)
	if err != nil {
		oktetoLog.Infof("error decoding resource on proxy: %s", err.Error())
		return nil, nil
	}

	// 2. Modify the object
	if err := t.modifyObject(obj); err != nil {
		return nil, err
	}

	// 3. Get encoder for the appropriate format based on Content-Type
	encoder := getEncoderForContentType(contentType)

	// 4. Encode back using the selected encoder
	var buf bytes.Buffer
	if err := encoder.Encode(obj, &buf); err != nil {
		oktetoLog.Infof("error encoding resource on proxy: %s", err.Error())
		return nil, fmt.Errorf("could not encode resource: %w", err)
	}

	return buf.Bytes(), nil
}

// getEncoderForContentType returns the appropriate encoder based on Content-Type header
func getEncoderForContentType(contentType string) runtime.Encoder {
	switch contentType {
	case "application/vnd.kubernetes.protobuf":
		// For protobuf, use the protobuf serializer
		return protobuf.NewSerializer(scheme.Scheme, scheme.Scheme)
	case "application/json", "application/apply-patch+yaml":
		// For JSON and YAML (server-side apply), use JSON encoder
		// Kubernetes YAML is just JSON in YAML syntax
		return json.NewSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, false)
	default:
		// Default to JSON encoding
		return json.NewSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, false)
	}
}

// modifyObject modifies the runtime.Object by adding labels and applying transformations
func (t *Translator) modifyObject(obj runtime.Object) error {
	// Add metadata labels
	if err := t.addMetadataLabels(obj); err != nil {
		return err
	}

	// Handle resource-specific transformations
	switch o := obj.(type) {
	case *appsv1.Deployment:
		return t.modifyPodTemplateSpec(&o.Spec.Template)
	case *appsv1.StatefulSet:
		return t.modifyPodTemplateSpec(&o.Spec.Template)
	case *batchv1.Job:
		return t.modifyPodTemplateSpec(&o.Spec.Template)
	case *batchv1.CronJob:
		return t.modifyPodTemplateSpec(&o.Spec.JobTemplate.Spec.Template)
	case *appsv1.DaemonSet:
		return t.modifyPodTemplateSpec(&o.Spec.Template)
	case *apiv1.ReplicationController:
		if o.Spec.Template != nil {
			return t.modifyPodTemplateSpec(o.Spec.Template)
		}
	case *appsv1.ReplicaSet:
		return t.modifyPodTemplateSpec(&o.Spec.Template)
	}

	return nil
}

// addMetadataLabels adds the deployed-by label to the object's metadata
func (t *Translator) addMetadataLabels(obj runtime.Object) error {
	accessor := meta.NewAccessor()

	// Get existing labels
	existingLabels, err := accessor.Labels(obj)
	if err != nil {
		return fmt.Errorf("could not get labels: %w", err)
	}

	// Add deployed-by label
	if existingLabels == nil {
		existingLabels = make(map[string]string)
	}
	existingLabels[model.DeployedByLabel] = t.name

	// Set labels back
	if err := accessor.SetLabels(obj, existingLabels); err != nil {
		return fmt.Errorf("could not set labels: %w", err)
	}

	// Get existing annotations
	existingAnnotations, err := accessor.Annotations(obj)
	if err != nil {
		return fmt.Errorf("could not get annotations: %w", err)
	}

	// Add okteto sample annotation
	if existingAnnotations == nil {
		existingAnnotations = make(map[string]string)
	}
	existingAnnotations[model.OktetoSampleAnnotation] = "true"

	// Set annotations back
	if err := accessor.SetAnnotations(obj, existingAnnotations); err != nil {
		return fmt.Errorf("could not set annotations: %w", err)
	}

	return nil
}

// modifyPodTemplateSpec modifies a PodTemplateSpec by adding labels and applying divert transformations
func (t *Translator) modifyPodTemplateSpec(template *apiv1.PodTemplateSpec) error {
	// Add labels to template metadata
	if template.Labels == nil {
		template.Labels = make(map[string]string)
	}
	template.Labels[model.DeployedByLabel] = t.name
	labels.SetInMetadata(&template.ObjectMeta, model.DeployedByLabel, t.name)

	// Apply divert transformations if driver is set
	if t.divertDriver != nil {
		template.Spec = t.divertDriver.UpdatePod(template.Spec)
	}

	return nil
}
