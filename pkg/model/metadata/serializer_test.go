// Copyright 2021 The Okteto Authors
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

package metadata

import (
	"os"
	"reflect"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestLabelsUnmashalling(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected Labels
	}{
		{
			"key-value-list",
			[]byte(`- env=production`),
			Labels{"env": "production"},
		},
		{
			"key-value-map",
			[]byte(`env: production`),
			Labels{"env": "production"},
		},
		{
			"key-value-complex-list",
			[]byte(`- env='production=11231231asa#$˜GADAFA'`),
			Labels{"env": "'production=11231231asa#$˜GADAFA'"},
		},
		{
			"key-value-with-env-var-list",
			[]byte(`- env=$DEV_ENV`),
			Labels{"env": "test_environment"},
		},
		{
			"key-value-with-env-var-map",
			[]byte(`env: $DEV_ENV`),
			Labels{"env": "test_environment"},
		},
		{
			"key-value-with-env-var-in-string-list",
			[]byte(`- env=my_env;$DEV_ENV;prod`),
			Labels{"env": "my_env;test_environment;prod"},
		},
		{
			"key-value-with-env-var-in-string-map",
			[]byte(`env: my_env;$DEV_ENV;prod`),
			Labels{"env": "my_env;test_environment;prod"},
		},
		{
			"simple-key-list",
			[]byte(`- noenv`),
			Labels{"noenv": ""},
		},
		{
			"key-with-no-value-list",
			[]byte(`- noenv=`),
			Labels{"noenv": ""},
		},
		{
			"key-with-no-value-map",
			[]byte(`noenv:`),
			Labels{"noenv": ""},
		},
		{
			"key-with-env-var-not-defined-list",
			[]byte(`- noenv=$UNDEFINED`),
			Labels{"noenv": ""},
		},
		{
			"key-with-env-var-not-defined-map",
			[]byte(`noenv: $UNDEFINED`),
			Labels{"noenv": ""},
		},
		{
			"just-env-var-list",
			[]byte(`- $DEV_ENV`),
			Labels{"test_environment": ""},
		},
		{
			"just-env-var-undefined-list",
			[]byte(`- $UNDEFINED`),
			Labels{"": ""},
		},
		{
			"local_env_expanded-list",
			[]byte(`- OKTETO_TEST_ENV_MARSHALLING`),
			Labels{"OKTETO_TEST_ENV_MARSHALLING": "true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make(Labels)
			if err := os.Setenv("DEV_ENV", "test_environment"); err != nil {
				t.Fatal(err)
			}

			if err := os.Setenv("OKTETO_TEST_ENV_MARSHALLING", "true"); err != nil {
				t.Fatal(err)
			}

			if err := yaml.UnmarshalStrict(tt.data, &result); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("didn't unmarshal correctly. Actual %+v, Expected %+v", result, tt.expected)
			}
		})
	}
}

func TestAnnotationsUnmashalling(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected Annotations
	}{
		{
			"key-value-list",
			[]byte(`- env=production`),
			Annotations{"env": "production"},
		},
		{
			"key-value-map",
			[]byte(`env: production`),
			Annotations{"env": "production"},
		},
		{
			"key-value-complex-list",
			[]byte(`- env='production=11231231asa#$˜GADAFA'`),
			Annotations{"env": "'production=11231231asa#$˜GADAFA'"},
		},
		{
			"key-value-with-env-var-list",
			[]byte(`- env=$DEV_ENV`),
			Annotations{"env": "test_environment"},
		},
		{
			"key-value-with-env-var-map",
			[]byte(`env: $DEV_ENV`),
			Annotations{"env": "test_environment"},
		},
		{
			"key-value-with-env-var-in-string-list",
			[]byte(`- env=my_env;$DEV_ENV;prod`),
			Annotations{"env": "my_env;test_environment;prod"},
		},
		{
			"key-value-with-env-var-in-string-map",
			[]byte(`env: my_env;$DEV_ENV;prod`),
			Annotations{"env": "my_env;test_environment;prod"},
		},
		{
			"simple-key-list",
			[]byte(`- noenv`),
			Annotations{"noenv": ""},
		},
		{
			"key-with-no-value-list",
			[]byte(`- noenv=`),
			Annotations{"noenv": ""},
		},
		{
			"key-with-no-value-map",
			[]byte(`noenv:`),
			Annotations{"noenv": ""},
		},
		{
			"key-with-env-var-not-defined-list",
			[]byte(`- noenv=$UNDEFINED`),
			Annotations{"noenv": ""},
		},
		{
			"key-with-env-var-not-defined-map",
			[]byte(`noenv: $UNDEFINED`),
			Annotations{"noenv": ""},
		},
		{
			"just-env-var-list",
			[]byte(`- $DEV_ENV`),
			Annotations{"test_environment": ""},
		},
		{
			"just-env-var-undefined-list",
			[]byte(`- $UNDEFINED`),
			Annotations{"": ""},
		},
		{
			"local_env_expanded-list",
			[]byte(`- OKTETO_TEST_ENV_MARSHALLING`),
			Annotations{"OKTETO_TEST_ENV_MARSHALLING": "true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make(Annotations)
			if err := os.Setenv("DEV_ENV", "test_environment"); err != nil {
				t.Fatal(err)
			}

			if err := os.Setenv("OKTETO_TEST_ENV_MARSHALLING", "true"); err != nil {
				t.Fatal(err)
			}

			if err := yaml.UnmarshalStrict(tt.data, &result); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("didn't unmarshal correctly. Actual %+v, Expected %+v", result, tt.expected)
			}
		})
	}
}
