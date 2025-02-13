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

	"github.com/okteto/okteto/cmd/utils"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/protobuf"
	"k8s.io/kubectl/pkg/scheme"
)

type ProtobufTranslator struct {
	b          []byte
	serializer *protobuf.Serializer
	name       string
}

func NewProtobufTranslator(b []byte, name string) *ProtobufTranslator {
	protobufSerializer := protobuf.NewSerializer(scheme.Scheme, scheme.Scheme)
	return &ProtobufTranslator{
		b:          b,
		name:       name,
		serializer: protobufSerializer,
	}
}

func (p *ProtobufTranslator) Translate() ([]byte, error) {
	obj, _, err := p.serializer.Decode(p.b, nil, nil)
	if err != nil {
		oktetoLog.Infof("error unmarshalling resource body on proxy: %s", err.Error())
		return nil, fmt.Errorf("could not unmarshal resource body: %w", err)
	}

	if err := p.translateMetadata(obj); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := p.serializer.Encode(obj, &buf); err != nil {
		return nil, fmt.Errorf("could not encode resource: %w", err)
	}

	return buf.Bytes(), nil
}

func (p *ProtobufTranslator) translateMetadata(obj runtime.Object) error {
	metaObj, err := meta.Accessor(obj)
	if err != nil {
		return fmt.Errorf("could not access object metadata: %w", err)
	}

	// Update labels directly instead of using labels.SetInMetadata.
	currentLabels := metaObj.GetLabels()
	if currentLabels == nil {
		currentLabels = make(map[string]string)
	}
	currentLabels[model.DeployedByLabel] = p.name
	metaObj.SetLabels(currentLabels)

	// Update annotations directly.
	annotations := metaObj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if utils.IsOktetoRepo() {
		annotations[model.OktetoSampleAnnotation] = "true"
	}
	metaObj.SetAnnotations(annotations)

	return nil
}
