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

package schema

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/kaptinlin/jsonschema"
	"github.com/stretchr/testify/assert"
)

func Test_build(t *testing.T) {
	oktetoJsonSchema, err := NewJsonSchema().ToJSON()
	assert.NoError(t, err)

	content := `
build:
  api:
    context: .
`

	var obj interface{}
	err = Unmarshal([]byte(content), &obj)
	//err = yaml.Unmarshal([]byte(content), &obj)
	assert.NoError(t, err)

	compiler := jsonschema.NewCompiler()

	//err = compiler.AddResource("okteto.json", strings.NewReader(exampleSchema))
	//assert.NoError(t, err)
	//schema, err := compiler.Compile("okteto.json")
	schema, err := compiler.Compile([]byte(oktetoJsonSchema))
	assert.NoError(t, err)
	result := schema.Validate(obj)

	//assert.Equal(t, true, result.ToFlag())
	if !result.IsValid() {
		details, _ := json.MarshalIndent(result.ToList(), "", "  ")
		fmt.Println(string(details))
	}
	evaluation := result.Error()

	assert.Equal(t, "", evaluation)
	//assert.Equal(t, true, result.IsValid())
	//assert.Nil(t, result.Error())
	//fmt.Println(result.Error())
}
