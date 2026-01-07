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
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/runtime/serializer/protobuf"
	"k8s.io/kubectl/pkg/scheme"
)

type protobufTranslator struct {
	protobufSerializer *protobuf.Serializer
	jsonSerializer     *json.Serializer
	jsonTranslator     *jsonTranslator
}

func newProtobufTranslator(name string, divertDriver divert.Driver) *protobufTranslator {
	return &protobufTranslator{
		protobufSerializer: protobuf.NewSerializer(scheme.Scheme, scheme.Scheme),
		jsonSerializer:     json.NewSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, false),
		jsonTranslator:     newJSONTranslator(name, divertDriver),
	}
}

// Translate converts Protobuf to JSON, processes it using the JSON translator, and converts back to Protobuf
func (p *protobufTranslator) Translate(b []byte) ([]byte, error) {
	// 1. Decode Protobuf bytes to runtime.Object
	obj, _, err := p.protobufSerializer.Decode(b, nil, nil)
	if err != nil {
		oktetoLog.Infof("error decoding protobuf on proxy: %s", err.Error())
		return nil, fmt.Errorf("could not decode protobuf: %w", err)
	}

	// 2. Encode runtime.Object to JSON
	var jsonBuf bytes.Buffer
	if err := p.jsonSerializer.Encode(obj, &jsonBuf); err != nil {
		oktetoLog.Infof("error encoding to JSON on proxy: %s", err.Error())
		return nil, fmt.Errorf("could not encode to JSON: %w", err)
	}

	// 3. Use the JSON translator to process the JSON
	processedJSON, err := p.jsonTranslator.Translate(jsonBuf.Bytes())
	if err != nil {
		return nil, err
	}

	if processedJSON == nil {
		return nil, nil
	}

	// 4. Decode processed JSON back to runtime.Object
	processedObj, _, err := p.jsonSerializer.Decode(processedJSON, nil, nil)
	if err != nil {
		oktetoLog.Infof("error decoding processed JSON on proxy: %s", err.Error())
		return nil, fmt.Errorf("could not decode processed JSON: %w", err)
	}

	// 5. Encode runtime.Object back to Protobuf
	var protobufBuf bytes.Buffer
	if err := p.protobufSerializer.Encode(processedObj, &protobufBuf); err != nil {
		oktetoLog.Infof("error encoding to protobuf on proxy: %s", err.Error())
		return nil, fmt.Errorf("could not encode to protobuf: %w", err)
	}

	return protobufBuf.Bytes(), nil
}
