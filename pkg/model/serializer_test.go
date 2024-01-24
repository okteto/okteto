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

package model

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/deps"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/externalresource"
	"github.com/okteto/okteto/pkg/model/forward"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

func TestReverseMarshalling(t *testing.T) {
	tests := []struct {
		name      string
		data      string
		expected  Reverse
		expectErr bool
	}{
		{
			name:     "basic",
			data:     "8080:9090",
			expected: Reverse{Local: 9090, Remote: 8080},
		},
		{
			name:     "equal",
			data:     "8080:8080",
			expected: Reverse{Local: 8080, Remote: 8080},
		},
		{
			name:      "missing-part",
			data:      "8080",
			expectErr: true,
		},
		{
			name:      "non-integer",
			data:      "8080:svc",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result Reverse
			if err := yaml.Unmarshal([]byte(tt.data), &result); err != nil {
				if tt.expectErr {
					return
				}

				t.Fatal(err)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("didn't unmarshal correctly. Actual '%+v', Expected '%+v'", result, tt.expected)
			}

			out, err := yaml.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}

			outStr := string(out)
			outStr = strings.TrimSuffix(outStr, "\n")

			if !reflect.DeepEqual(outStr, tt.data) {
				t.Errorf("didn't unmarshal correctly. Actual '%+v', Expected '%+v'", outStr, tt.data)
			}
		})
	}
}

func TestEnvVarMarshalling(t *testing.T) {
	tests := []struct {
		expected env.Var
		name     string
		data     []byte
	}{
		{
			name:     "key-value",
			data:     []byte(`env=production`),
			expected: env.Var{Name: "env", Value: "production"},
		},
		{
			name:     "key-value-complex",
			data:     []byte(`env='production=11231231asa#$˜GADAFA'`),
			expected: env.Var{Name: "env", Value: "'production=11231231asa#$˜GADAFA'"},
		},
		{
			name:     "key-value-with-env-var",
			data:     []byte(`env=$DEV_ENV`),
			expected: env.Var{Name: "env", Value: "test_environment"},
		},
		{
			name:     "key-value-with-env-var-in-string",
			data:     []byte(`env=my_env;$DEV_ENV;prod`),
			expected: env.Var{Name: "env", Value: "my_env;test_environment;prod"},
		},
		{
			name:     "simple-key",
			data:     []byte(`noenv`),
			expected: env.Var{Name: "noenv", Value: ""},
		},
		{
			name:     "key-with-no-value",
			data:     []byte(`noenv=`),
			expected: env.Var{Name: "noenv", Value: ""},
		},
		{
			name:     "key-with-env-var-not-defined",
			data:     []byte(`noenv=$UNDEFINED`),
			expected: env.Var{Name: "noenv", Value: ""},
		},
		{
			name:     "just-env-var",
			data:     []byte(`$DEV_ENV`),
			expected: env.Var{Name: "test_environment", Value: ""},
		},
		{
			name:     "just-env-var-undefined",
			data:     []byte(`$UNDEFINED`),
			expected: env.Var{Name: "", Value: ""},
		},
		{
			name:     "local_env_expanded",
			data:     []byte(`OKTETO_TEST_ENV_MARSHALLING`),
			expected: env.Var{Name: "OKTETO_TEST_ENV_MARSHALLING", Value: "true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var result env.Var
			t.Setenv("DEV_ENV", "test_environment")
			t.Setenv("OKTETO_TEST_ENV_MARSHALLING", "true")

			if err := yaml.Unmarshal(tt.data, &result); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("didn't unmarshal correctly. Actual %+v, Expected %+v", result, tt.expected)
			}

			_, err := yaml.Marshal(&result)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCommandUnmarshalling(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected Command
	}{
		{
			"single-no-space",
			[]byte("start.sh"),
			Command{Values: []string{"start.sh"}},
		},
		{
			"single-space",
			[]byte("start.sh arg"),
			Command{Values: []string{"sh", "-c", "start.sh arg"}},
		},
		{
			"double-command",
			[]byte("mkdir myproject && cd myproject"),
			Command{Values: []string{"sh", "-c", "mkdir myproject && cd myproject"}},
		},
		{
			"multiple",
			[]byte("['yarn', 'install']"),
			Command{Values: []string{"yarn", "install"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var result Command
			if err := yaml.Unmarshal(tt.data, &result); err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHybridCommandUnmarshalling(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected hybridCommand
	}{
		{
			"single-no-space",
			[]byte("start.sh"),
			hybridCommand{Values: []string{"start.sh"}},
		},
		{
			"single-space",
			[]byte("start.sh arg"),
			hybridCommand{Values: []string{"start.sh", "arg"}},
		},
		{
			"double-command",
			[]byte("mkdir myproject && cd myproject"),
			hybridCommand{Values: []string{"mkdir", "myproject", "&&", "cd", "myproject"}},
		},
		{
			"multiple",
			[]byte("['yarn', 'install']"),
			hybridCommand{Values: []string{"yarn", "install"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var result hybridCommand
			if err := yaml.Unmarshal(tt.data, &result); err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCommandMarshalling(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		command  Command
	}{
		{
			name:     "single-command",
			command:  Command{Values: []string{"bash"}},
			expected: "bash\n",
		},
		{
			name:     "multiple-command",
			command:  Command{Values: []string{"yarn", "start"}},
			expected: "- yarn\n- start\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marshalled, err := yaml.Marshal(tt.command)
			if err != nil {
				t.Fatal(err)
			}

			if string(marshalled) != tt.expected {
				t.Errorf("didn't marshal correctly. Actual %s, Expected %s", marshalled, tt.expected)
			}
		})
	}
}

func TestImageMarshalling(t *testing.T) {
	tests := []struct {
		name     string
		image    *build.Info
		expected string
	}{
		{
			name:     "single-name",
			image:    &build.Info{Name: "image-name"},
			expected: "image-name\n",
		},
		{
			name:     "single-name-and-defaults",
			image:    &build.Info{Name: "image-name", Context: "."},
			expected: "image-name\n",
		},
		{
			name:     "build",
			image:    &build.Info{Name: "image-name", Context: "path"},
			expected: "name: image-name\ncontext: path\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marshalled, err := yaml.Marshal(tt.image)
			if err != nil {
				t.Fatal(err)
			}

			if string(marshalled) != tt.expected {
				t.Errorf("didn't marshal correctly. Actual %s, Expected %s", marshalled, tt.expected)
			}
		})
	}
}

func TestProbesMarshalling(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		probes   Probes
	}{
		{
			name:     "liveness-true-and-defaults",
			probes:   Probes{Liveness: true},
			expected: "liveness: true\n",
		},
		{
			name:     "all-probes-true",
			probes:   Probes{Liveness: true, Readiness: true, Startup: true},
			expected: "true\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marshalled, err := yaml.Marshal(tt.probes)
			if err != nil {
				t.Fatal(err)
			}

			if string(marshalled) != tt.expected {
				t.Errorf("didn't marshal correctly. Actual '%s', Expected '%s'", marshalled, tt.expected)
			}
		})
	}
}

func TestLifecycleMarshalling(t *testing.T) {
	tests := []struct {
		name      string
		expected  string
		lifecycle Lifecycle
	}{
		{
			name:      "true-and-false",
			lifecycle: Lifecycle{PostStart: true},
			expected:  "postStart: true\n",
		},
		{
			name:      "all-lifecycle-true",
			lifecycle: Lifecycle{PostStart: true, PostStop: true},
			expected:  "true\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marshalled, err := yaml.Marshal(tt.lifecycle)
			if err != nil {
				t.Fatal(err)
			}

			if string(marshalled) != tt.expected {
				t.Errorf("didn't marshal correctly. Actual %s, Expected %s", marshalled, tt.expected)
			}
		})
	}
}

func TestSecretMarshalling(t *testing.T) {
	file, err := os.CreateTemp("", "okteto-secret-test")
	assert.NoError(t, err)
	defer os.Remove(file.Name())

	t.Setenv("TEST_HOME", file.Name())

	tests := []struct {
		expected      *Secret
		name          string
		data          string
		expectedError bool
	}{
		{
			name:          "local:remote",
			data:          fmt.Sprintf("%s:/remote", file.Name()),
			expected:      &Secret{LocalPath: file.Name(), RemotePath: "/remote", Mode: 420},
			expectedError: false,
		},
		{
			name:          "local:remote:mode",
			data:          fmt.Sprintf("%s:/remote:400", file.Name()),
			expected:      &Secret{LocalPath: file.Name(), RemotePath: "/remote", Mode: 256},
			expectedError: false,
		},
		{
			name:          "variables",
			data:          "$TEST_HOME:/remote",
			expected:      &Secret{LocalPath: file.Name(), RemotePath: "/remote", Mode: 420},
			expectedError: false,
		},
		{
			name:          "too-short",
			data:          "local",
			expected:      &Secret{LocalPath: "", RemotePath: "", Mode: 0},
			expectedError: false,
		},
		{
			name:          "too-long",
			data:          "local:remote:mode:other",
			expected:      &Secret{LocalPath: "", RemotePath: "", Mode: 0},
			expectedError: false,
		},
		{
			name:          "wrong-local",
			data:          "/local:/remote:400",
			expected:      &Secret{LocalPath: "/local", RemotePath: "/remote", Mode: 256},
			expectedError: false,
		},
		{
			name:          "wrong-remote",
			data:          fmt.Sprintf("%s:remote", file.Name()),
			expected:      &Secret{LocalPath: file.Name(), RemotePath: "remote", Mode: 420},
			expectedError: false,
		},
		{
			name:          "wrong-mode",
			data:          fmt.Sprintf("%s:/remote:aaa", file.Name()),
			expected:      nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result Secret
			if err := yaml.Unmarshal([]byte(tt.data), &result); err != nil {
				if !tt.expectedError {
					t.Fatalf("unexpected error unmarshaling %s: %s", tt.name, err.Error())
				}
				return
			}
			if tt.expectedError {
				t.Fatalf("expected error unmarshaling %s not thrown", tt.name)
			}
			if result.LocalPath != tt.expected.LocalPath {
				t.Errorf("didn't unmarshal correctly LocalPath. Actual %s, Expected %s", result.LocalPath, tt.expected.LocalPath)
			}
			if result.RemotePath != tt.expected.RemotePath {
				t.Errorf("didn't unmarshal correctly RemotePath. Actual %s, Expected %s", result.RemotePath, tt.expected.RemotePath)
			}
			if result.Mode != tt.expected.Mode {
				t.Errorf("didn't unmarshal correctly Mode. Actual %d, Expected %d", result.Mode, tt.expected.Mode)
			}

			_, err := yaml.Marshal(&result)
			if err != nil {
				t.Fatalf("error marshaling %s: %s", tt.name, err)
			}
		})
	}
}

func TestVolumeMarshalling(t *testing.T) {
	tests := []struct {
		expected Volume
		name     string
		data     []byte
	}{
		{
			name:     "global",
			data:     []byte("/path"),
			expected: Volume{LocalPath: "", RemotePath: "/path"},
		},
		{
			name:     "relative",
			data:     []byte("sub:/path"),
			expected: Volume{LocalPath: "sub", RemotePath: "/path"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v Volume
			if err := yaml.Unmarshal(tt.data, &v); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(v, tt.expected) {
				t.Errorf("didn't unmarshal correctly. Actual %s, Expected %s", v, tt.expected)
			}

			_, err := yaml.Marshal(&v)
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDevMarshalling(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		dev      Dev
	}{
		{
			name:     "healtcheck-not-defaults",
			dev:      Dev{Name: "name-test", Probes: &Probes{Liveness: true}},
			expected: "probes:\n  liveness: true\nname: name-test\n",
		},
		{
			name:     "healtcheck-all-true-by-healthchecks",
			dev:      Dev{Name: "name-test", Healthchecks: true},
			expected: "name: name-test\nhealthchecks: true\n",
		},
		{
			name:     "healtcheck-all-true-by-probes",
			dev:      Dev{Name: "name-test", Probes: &Probes{Liveness: true, Readiness: true, Startup: true}},
			expected: "name: name-test\nhealthchecks: true\n",
		},
		{
			name:     "pv-enabled-not-show-after-marshall",
			dev:      Dev{Name: "name-test", PersistentVolumeInfo: &PersistentVolumeInfo{Enabled: true}},
			expected: "name: name-test\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marshalled, err := yaml.Marshal(&tt.dev)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, tt.expected, string(marshalled))
		})
	}
}

func TestEndpointUnmarshalling(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected Endpoint
	}{
		{
			name: "rule",
			data: []byte("- path: /\n  service: test\n  port: 8080"),
			expected: Endpoint{
				Rules: []EndpointRule{{
					Path:    "/",
					Service: "test",
					Port:    8080,
				}},
				Labels:      Labels{},
				Annotations: make(Annotations),
			},
		},
		{
			name: "full-endpoint",
			data: []byte("labels:\n  key1: value1\nannotations:\n  key2: value2\nrules:\n- path: /\n  service: test\n  port: 8080"),
			expected: Endpoint{
				Labels:      Labels{},
				Annotations: Annotations{"key1": "value1", "key2": "value2"},
				Rules: []EndpointRule{{
					Path:    "/",
					Service: "test",
					Port:    8080,
				}},
			},
		},
		{
			name: "full-endpoint without labels",
			data: []byte("annotations:\n  key2: value2\nrules:\n- path: /\n  service: test\n  port: 8080"),
			expected: Endpoint{
				Labels:      Labels{},
				Annotations: Annotations{"key2": "value2"},
				Rules: []EndpointRule{{
					Path:    "/",
					Service: "test",
					Port:    8080,
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var endpoint Endpoint
			if err := yaml.UnmarshalStrict(tt.data, &endpoint); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(endpoint.Annotations, tt.expected.Annotations) {
				t.Errorf("didn't unmarshal correctly annotations. Actual %v, Expected %v", endpoint.Annotations, tt.expected.Annotations)
			}

			if !reflect.DeepEqual(endpoint.Labels, tt.expected.Labels) {
				t.Errorf("didn't unmarshal correctly labels. Actual %v, Expected %v", endpoint.Labels, tt.expected.Labels)
			}

			if !reflect.DeepEqual(endpoint.Rules, tt.expected.Rules) {
				t.Errorf("didn't unmarshal correctly rules. Actual %v, Expected %v", endpoint.Rules, tt.expected.Rules)
			}
		})
	}
}

func TestLabelsUnmarshalling(t *testing.T) {
	tests := []struct {
		expected Labels
		name     string
		data     []byte
	}{
		{
			name:     "key-value-with-env-var-map",
			data:     []byte(`env: $DEV_ENV`),
			expected: Labels{"env": "test_environment"},
		},
		{
			name:     "key-value-list",
			data:     []byte(`- env=production`),
			expected: Labels{"env": "production"},
		},
		{
			name:     "key-value-map",
			data:     []byte(`env: production`),
			expected: Labels{"env": "production"},
		},
		{
			name:     "key-value-complex-list",
			data:     []byte(`- env='production=11231231asa#$˜GADAFA'`),
			expected: Labels{"env": "'production=11231231asa#$˜GADAFA'"},
		},
		{
			name:     "key-value-with-env-var-list",
			data:     []byte(`- env=$DEV_ENV`),
			expected: Labels{"env": "test_environment"},
		},
		{
			name:     "key-value-with-env-var-map",
			data:     []byte(`env: $DEV_ENV`),
			expected: Labels{"env": "test_environment"},
		},
		{
			name:     "key-value-with-env-var-in-string-list",
			data:     []byte(`- env=my_env;$DEV_ENV;prod`),
			expected: Labels{"env": "my_env;test_environment;prod"},
		},
		{
			name:     "key-value-with-env-var-in-string-map",
			data:     []byte(`env: my_env;$DEV_ENV;prod`),
			expected: Labels{"env": "my_env;test_environment;prod"},
		},
		{
			name:     "simple-key-list",
			data:     []byte(`- noenv`),
			expected: Labels{"noenv": ""},
		},
		{
			name:     "key-with-no-value-list",
			data:     []byte(`- noenv=`),
			expected: Labels{"noenv": ""},
		},
		{
			name:     "key-with-no-value-map",
			data:     []byte(`noenv:`),
			expected: Labels{"noenv": ""},
		},
		{
			name:     "key-with-env-var-not-defined-list",
			data:     []byte(`- noenv=$UNDEFINED`),
			expected: Labels{"noenv": ""},
		},
		{
			name:     "key-with-env-var-not-defined-map",
			data:     []byte(`noenv: $UNDEFINED`),
			expected: Labels{"noenv": ""},
		},
		{
			name:     "just-env-var-list",
			data:     []byte(`- $DEV_ENV`),
			expected: Labels{"test_environment": ""},
		},
		{
			name:     "just-env-var-undefined-list",
			data:     []byte(`- $UNDEFINED`),
			expected: Labels{"": ""},
		},
		{
			name:     "local_env_expanded-list",
			data:     []byte(`- OKTETO_TEST_ENV_MARSHALLING`),
			expected: Labels{"OKTETO_TEST_ENV_MARSHALLING": "true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make(Labels)

			t.Setenv("DEV_ENV", "test_environment")
			t.Setenv("OKTETO_TEST_ENV_MARSHALLING", "true")

			if err := yaml.UnmarshalStrict(tt.data, &result); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("didn't unmarshal correctly. Actual %+v, Expected %+v", result, tt.expected)
			}
		})
	}
}

func TestAnnotationsUnmarshalling(t *testing.T) {
	tests := []struct {
		expected Annotations
		name     string
		data     []byte
	}{
		{
			name:     "key-value-list",
			data:     []byte(`- env=production`),
			expected: Annotations{"env": "production"},
		},
		{
			name:     "key-value-map",
			data:     []byte(`env: production`),
			expected: Annotations{"env": "production"},
		},
		{
			name:     "key-value-complex-list",
			data:     []byte(`- env='production=11231231asa#$˜GADAFA'`),
			expected: Annotations{"env": "'production=11231231asa#$˜GADAFA'"},
		},
		{
			name:     "key-value-with-env-var-list",
			data:     []byte(`- env=$DEV_ENV`),
			expected: Annotations{"env": "test_environment"},
		},
		{
			name:     "key-value-with-env-var-map",
			data:     []byte(`env: $DEV_ENV`),
			expected: Annotations{"env": "test_environment"},
		},
		{
			name:     "key-value-with-env-var-in-string-list",
			data:     []byte(`- env=my_env;$DEV_ENV;prod`),
			expected: Annotations{"env": "my_env;test_environment;prod"},
		},
		{
			name:     "key-value-with-env-var-in-string-map",
			data:     []byte(`env: my_env;$DEV_ENV;prod`),
			expected: Annotations{"env": "my_env;test_environment;prod"},
		},
		{
			name:     "simple-key-list",
			data:     []byte(`- noenv`),
			expected: Annotations{"noenv": ""},
		},
		{
			name:     "key-with-no-value-list",
			data:     []byte(`- noenv=`),
			expected: Annotations{"noenv": ""},
		},
		{
			name:     "key-with-no-value-map",
			data:     []byte(`noenv:`),
			expected: Annotations{"noenv": ""},
		},
		{
			name:     "key-with-env-var-not-defined-list",
			data:     []byte(`- noenv=$UNDEFINED`),
			expected: Annotations{"noenv": ""},
		},
		{
			name:     "key-with-env-var-not-defined-map",
			data:     []byte(`noenv: $UNDEFINED`),
			expected: Annotations{"noenv": ""},
		},
		{
			name:     "just-env-var-list",
			data:     []byte(`- $DEV_ENV`),
			expected: Annotations{"test_environment": ""},
		},
		{
			name:     "just-env-var-undefined-list",
			data:     []byte(`- $UNDEFINED`),
			expected: Annotations{"": ""},
		},
		{
			name:     "local_env_expanded-list",
			data:     []byte(`- OKTETO_TEST_ENV_MARSHALLING`),
			expected: Annotations{"OKTETO_TEST_ENV_MARSHALLING": "true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make(Annotations)

			t.Setenv("DEV_ENV", "test_environment")
			t.Setenv("OKTETO_TEST_ENV_MARSHALLING", "true")

			if err := yaml.UnmarshalStrict(tt.data, &result); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("didn't unmarshal correctly. Actual %+v, Expected %+v", result, tt.expected)
			}
		})
	}
}

func TestDurationUnmarshalling(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected Duration
	}{
		{
			name:     "No units",
			data:     []byte(`10`),
			expected: Duration(10 * time.Second),
		},
		{
			name:     "Only one unit",
			data:     []byte(`10s`),
			expected: Duration(10 * time.Second),
		},
		{
			name:     "Complex units",
			data:     []byte(`1m10s`),
			expected: Duration(70 * time.Second),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Duration(0)

			if err := yaml.UnmarshalStrict(tt.data, &result); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("didn't unmarshal correctly. Actual %+v, Expected %+v", result, tt.expected)
			}
		})
	}
}

func TestTimeoutUnmarshalling(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected Timeout
	}{
		{
			name:     "Direct default",
			data:     []byte(`10`),
			expected: Timeout{Default: 10 * time.Second},
		},
		{
			name: "only default ",
			data: []byte(`
default: 30s
`),
			expected: Timeout{Default: 30 * time.Second},
		},
		{
			name: "only resources",
			data: []byte(`
resources: 30s
`),
			expected: Timeout{Resources: 30 * time.Second},
		},
		{
			name: "both set",
			data: []byte(`
default: 10s
resources: 30s
`),
			expected: Timeout{Default: 10 * time.Second, Resources: 30 * time.Second},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Timeout{}

			if err := yaml.UnmarshalStrict(tt.data, &result); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("didn't unmarshal correctly. Actual %+v, Expected %+v", result, tt.expected)
			}
		})
	}
}

func TestSyncUnmarshalling(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected Sync
	}{
		{
			name: "only-folders",
			data: []byte(`- .:/usr/src/app`),
			expected: Sync{
				Folders: []SyncFolder{
					{
						LocalPath:  ".",
						RemotePath: "/usr/src/app"},
				},
				Compression:    true,
				Verbose:        false,
				RescanInterval: DefaultSyncthingRescanInterval,
			},
		},
		{
			name: "all",
			data: []byte(`folders:
  - .:/usr/src/app
compression: false
verbose: true
rescanInterval: 10`),
			expected: Sync{
				Folders: []SyncFolder{
					{
						LocalPath:  ".",
						RemotePath: "/usr/src/app"},
				},
				Compression:    false,
				Verbose:        true,
				RescanInterval: 10,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Sync{}

			if err := yaml.UnmarshalStrict(tt.data, &result); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("didn't unmarshal correctly. Actual %+v, Expected %+v", result, tt.expected)
			}
		})
	}
}

func TestSyncFoldersUnmarshalling(t *testing.T) {
	t.Setenv("REMOTE_PATH", "/usr/src/app")
	tests := []struct {
		expected SyncFolder
		name     string
		data     []byte
	}{
		{
			name:     "same dir",
			data:     []byte(`.:/usr/src/app`),
			expected: SyncFolder{LocalPath: ".", RemotePath: "/usr/src/app"},
		},
		{
			name:     "same dir",
			data:     []byte(`.:${REMOTE_PATH}`),
			expected: SyncFolder{LocalPath: ".", RemotePath: "/usr/src/app"},
		},
		{
			name:     "previous dir",
			data:     []byte(`../:/usr/src/app`),
			expected: SyncFolder{LocalPath: "../", RemotePath: "/usr/src/app"},
		},
		{
			name:     "fullpath",
			data:     []byte(`/usr/src/app:/usr/src/app`),
			expected: SyncFolder{LocalPath: "/usr/src/app", RemotePath: "/usr/src/app"},
		},
		{
			name:     "windows test",
			data:     []byte(`C:/Users/src/test:/usr/src/app`),
			expected: SyncFolder{LocalPath: "C:/Users/src/test", RemotePath: "/usr/src/app"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SyncFolder{}

			if err := yaml.UnmarshalStrict(tt.data, &result); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("didn't unmarshal correctly. Actual %+v, Expected %+v", result, tt.expected)
			}
		})
	}
}

func TestManifestUnmarshalling(t *testing.T) {
	tests := []struct {
		expected        *Manifest
		name            string
		manifest        []byte
		isErrorExpected bool
	}{
		{
			name: "only dev with service unsupported field",
			manifest: []byte(`
sync:
  - app:/app
services:
  - name: svc
    autocreate: true`),
			expected:        nil,
			isErrorExpected: true,
		},
		{
			name: "manifest with namespace and context",
			manifest: []byte(`
namespace: test
context: context-to-use
deploy:
  - okteto stack deploy`),
			expected: &Manifest{
				Namespace: "test",
				Build:     map[string]*build.Info{},
				Deploy: &DeployInfo{
					Commands: []DeployCommand{
						{
							Name:    "okteto stack deploy",
							Command: "okteto stack deploy",
						},
					},
				},
				Destroy:      &DestroyInfo{},
				Dev:          map[string]*Dev{},
				Dependencies: map[string]*deps.Dependency{},
				External:     externalresource.Section{},
				Context:      "context-to-use",
				IsV2:         true,
				Type:         OktetoManifestType,
			},
			isErrorExpected: false,
		},
		{
			name: "dev manifest with dev sanitized and deploy",
			manifest: []byte(`
deploy:
  - okteto stack deploy
dev:
  test-1:
    sync:
    - app:/app
  test_2:
    sync:
    - app:/app
`),
			expected: &Manifest{
				IsV2:  true,
				Type:  OktetoManifestType,
				Build: map[string]*build.Info{},
				Deploy: &DeployInfo{
					Commands: []DeployCommand{
						{
							Name:    "okteto stack deploy",
							Command: "okteto stack deploy",
						},
					},
				},
				Destroy:      &DestroyInfo{},
				Dependencies: map[string]*deps.Dependency{},
				External:     externalresource.Section{},
				Dev: map[string]*Dev{
					"test-1": {
						Mode: constants.OktetoSyncModeFieldValue,
						Name: "test-1",
						Sync: Sync{
							RescanInterval: 300,
							Compression:    true,
							Folders: []SyncFolder{
								{
									LocalPath:  "app",
									RemotePath: "/app",
								},
							},
						},
						Forward:         []forward.Forward{},
						Selector:        Selector{},
						EmptyImage:      true,
						ImagePullPolicy: v1.PullAlways,
						Image: &build.Info{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Interface: Localhost,
						PersistentVolumeInfo: &PersistentVolumeInfo{
							Enabled: true,
						},
						Push: &build.Info{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Secrets: make([]Secret, 0),
						Command: Command{Values: []string{"sh"}},
						Probes: &Probes{
							Liveness:  false,
							Readiness: false,
							Startup:   false,
						},
						Lifecycle: &Lifecycle{
							PostStart: false,
							PostStop:  false,
						},
						SecurityContext: &SecurityContext{
							RunAsUser:    pointer.Int64(0),
							RunAsGroup:   pointer.Int64(0),
							RunAsNonRoot: nil,
							FSGroup:      pointer.Int64(0),
						},
						SSHServerPort: 2222,
						Services:      []*Dev{},
						InitContainer: InitContainer{
							Image: OktetoBinImageTag,
						},
						Timeout: Timeout{
							Resources: 120 * time.Second,
							Default:   60 * time.Second,
						},
						Metadata: &Metadata{
							Labels:      Labels{},
							Annotations: Annotations{},
						},
						Environment: env.Environment{},
						Volumes:     []Volume{},
					},
					"test-2": {
						Name: "test-2",
						Sync: Sync{
							RescanInterval: 300,
							Compression:    true,
							Folders: []SyncFolder{
								{
									LocalPath:  "app",
									RemotePath: "/app",
								},
							},
						},
						Forward:         []forward.Forward{},
						Selector:        Selector{},
						EmptyImage:      true,
						ImagePullPolicy: v1.PullAlways,
						Image: &build.Info{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Interface: Localhost,
						PersistentVolumeInfo: &PersistentVolumeInfo{
							Enabled: true,
						},
						Push: &build.Info{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Secrets: make([]Secret, 0),
						Command: Command{Values: []string{"sh"}},
						Probes: &Probes{
							Liveness:  false,
							Readiness: false,
							Startup:   false,
						},
						Lifecycle: &Lifecycle{
							PostStart: false,
							PostStop:  false,
						},
						SecurityContext: &SecurityContext{
							RunAsUser:    pointer.Int64(0),
							RunAsGroup:   pointer.Int64(0),
							RunAsNonRoot: nil,
							FSGroup:      pointer.Int64(0),
						},
						SSHServerPort: 2222,
						Services:      []*Dev{},
						InitContainer: InitContainer{
							Image: OktetoBinImageTag,
						},
						Timeout: Timeout{
							Resources: 120 * time.Second,
							Default:   60 * time.Second,
						},
						Metadata: &Metadata{
							Labels:      Labels{},
							Annotations: Annotations{},
						},
						Environment: env.Environment{},
						Volumes:     []Volume{},
						Mode:        constants.OktetoSyncModeFieldValue,
					},
				},
			},

			isErrorExpected: false,
		},
		{
			name: "only dev",
			manifest: []byte(`name: test
sync:
  - app:/app`),
			expected: &Manifest{
				Type:          OktetoManifestType,
				Build:         map[string]*build.Info{},
				Deploy:        &DeployInfo{},
				Destroy:       &DestroyInfo{},
				Dependencies:  map[string]*deps.Dependency{},
				External:      externalresource.Section{},
				GlobalForward: []forward.GlobalForward{},
				Dev: map[string]*Dev{
					"test": {
						Name: "test",
						Sync: Sync{
							RescanInterval: 300,
							Compression:    true,
							Folders: []SyncFolder{
								{
									LocalPath:  "app",
									RemotePath: "/app",
								},
							},
						},
						Forward:         []forward.Forward{},
						Selector:        Selector{},
						EmptyImage:      true,
						ImagePullPolicy: v1.PullAlways,
						Image: &build.Info{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Interface: Localhost,
						PersistentVolumeInfo: &PersistentVolumeInfo{
							Enabled: true,
						},
						Push: &build.Info{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Secrets: make([]Secret, 0),
						Command: Command{Values: []string{"sh"}},
						Probes: &Probes{
							Liveness:  false,
							Readiness: false,
							Startup:   false,
						},
						Lifecycle: &Lifecycle{
							PostStart: false,
							PostStop:  false,
						},
						SecurityContext: &SecurityContext{
							RunAsUser:    pointer.Int64(0),
							RunAsGroup:   pointer.Int64(0),
							RunAsNonRoot: nil,
							FSGroup:      pointer.Int64(0),
						},
						SSHServerPort: 2222,
						Services:      []*Dev{},
						InitContainer: InitContainer{
							Image: OktetoBinImageTag,
						},
						Timeout: Timeout{
							Resources: 120 * time.Second,
							Default:   60 * time.Second,
						},
						Metadata: &Metadata{
							Labels:      Labels{},
							Annotations: Annotations{},
						},
						Environment: env.Environment{},
						Volumes:     []Volume{},
						Mode:        constants.OktetoSyncModeFieldValue,
					},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "only dev with service",
			manifest: []byte(`name: test
sync:
  - app:/app
services:
  - name: svc`),
			expected: &Manifest{
				Type:          OktetoManifestType,
				Build:         map[string]*build.Info{},
				Deploy:        &DeployInfo{},
				Destroy:       &DestroyInfo{},
				Dependencies:  map[string]*deps.Dependency{},
				GlobalForward: []forward.GlobalForward{},
				External:      externalresource.Section{},
				Dev: map[string]*Dev{
					"test": {
						Name: "test",
						Sync: Sync{
							RescanInterval: 300,
							Compression:    true,
							Folders: []SyncFolder{
								{
									LocalPath:  "app",
									RemotePath: "/app",
								},
							},
						},
						Forward:         []forward.Forward{},
						Selector:        Selector{},
						EmptyImage:      true,
						ImagePullPolicy: v1.PullAlways,
						Image: &build.Info{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Interface: Localhost,
						PersistentVolumeInfo: &PersistentVolumeInfo{
							Enabled: true,
						},
						Push: &build.Info{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Secrets: make([]Secret, 0),
						Command: Command{Values: []string{"sh"}},
						Probes: &Probes{
							Liveness:  false,
							Readiness: false,
							Startup:   false,
						},
						Lifecycle: &Lifecycle{
							PostStart: false,
							PostStop:  false,
						},
						SecurityContext: &SecurityContext{
							RunAsUser:    pointer.Int64(0),
							RunAsGroup:   pointer.Int64(0),
							RunAsNonRoot: nil,
							FSGroup:      pointer.Int64(0),
						},
						SSHServerPort: 2222,
						Services: []*Dev{
							{
								Name:            "svc",
								Annotations:     Annotations{},
								Selector:        Selector{},
								EmptyImage:      true,
								Image:           &build.Info{},
								ImagePullPolicy: v1.PullAlways,
								Secrets:         []Secret{},
								Probes: &Probes{
									Liveness:  false,
									Readiness: false,
									Startup:   false,
								},
								Lifecycle: &Lifecycle{
									PostStart: false,
									PostStop:  false,
								},
								SecurityContext: &SecurityContext{
									RunAsUser:    pointer.Int64(0),
									RunAsGroup:   pointer.Int64(0),
									RunAsNonRoot: nil,
									FSGroup:      pointer.Int64(0),
								},
								Sync: Sync{
									RescanInterval: 300,
								},
								Forward:  []forward.Forward{},
								Reverse:  []Reverse{},
								Services: []*Dev{},
								Metadata: &Metadata{
									Labels:      Labels{},
									Annotations: Annotations{},
								},
								Volumes: []Volume{},
								Mode:    constants.OktetoSyncModeFieldValue,
							},
						},
						InitContainer: InitContainer{
							Image: OktetoBinImageTag,
						},
						Timeout: Timeout{
							Resources: 120 * time.Second,
							Default:   60 * time.Second,
						},
						Metadata: &Metadata{
							Labels:      Labels{},
							Annotations: Annotations{},
						},
						Environment: env.Environment{},
						Volumes:     []Volume{},
						Mode:        constants.OktetoSyncModeFieldValue,
					},
				},
			},
			isErrorExpected: false,
		},

		{
			name: "only dev with errors",
			manifest: []byte(`
sync:
  - app:/app
non-found-field:
  testing`),
			expected:        nil,
			isErrorExpected: true,
		},
		{
			name: "dev manifest with one dev",
			manifest: []byte(`
dev:
  test:
    sync:
    - app:/app
`),
			expected: &Manifest{
				Type:         OktetoManifestType,
				IsV2:         true,
				Build:        map[string]*build.Info{},
				Dependencies: map[string]*deps.Dependency{},
				External:     externalresource.Section{},
				Destroy:      &DestroyInfo{},
				Dev: map[string]*Dev{
					"test": {
						Name: "test",
						Sync: Sync{
							RescanInterval: 300,
							Compression:    true,
							Folders: []SyncFolder{
								{
									LocalPath:  "app",
									RemotePath: "/app",
								},
							},
						},
						Forward:         []forward.Forward{},
						Selector:        Selector{},
						EmptyImage:      true,
						ImagePullPolicy: v1.PullAlways,
						Image: &build.Info{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Interface: Localhost,
						PersistentVolumeInfo: &PersistentVolumeInfo{
							Enabled: true,
						},
						Secrets: make([]Secret, 0),
						Push: &build.Info{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Command: Command{Values: []string{"sh"}},
						Probes: &Probes{
							Liveness:  false,
							Readiness: false,
							Startup:   false,
						},
						Lifecycle: &Lifecycle{
							PostStart: false,
							PostStop:  false,
						},
						SecurityContext: &SecurityContext{
							RunAsUser:    pointer.Int64(0),
							RunAsGroup:   pointer.Int64(0),
							RunAsNonRoot: nil,
							FSGroup:      pointer.Int64(0),
						},
						SSHServerPort: 2222,
						Services:      []*Dev{},
						InitContainer: InitContainer{
							Image: OktetoBinImageTag,
						},
						Timeout: Timeout{
							Resources: 120 * time.Second,
							Default:   60 * time.Second,
						},
						Metadata: &Metadata{
							Labels:      Labels{},
							Annotations: Annotations{},
						},
						Environment: env.Environment{},
						Volumes:     []Volume{},
						Mode:        constants.OktetoSyncModeFieldValue,
					},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "dev manifest with multiple devs",
			manifest: []byte(`
dev:
  test-1:
    sync:
    - app:/app
  test-2:
    sync:
    - app:/app
`),
			expected: &Manifest{
				Type:         OktetoManifestType,
				IsV2:         true,
				Build:        map[string]*build.Info{},
				Dependencies: map[string]*deps.Dependency{},
				External:     externalresource.Section{},
				Destroy:      &DestroyInfo{},
				Dev: map[string]*Dev{
					"test-1": {
						Name: "test-1",
						Sync: Sync{
							RescanInterval: 300,
							Compression:    true,
							Folders: []SyncFolder{
								{
									LocalPath:  "app",
									RemotePath: "/app",
								},
							},
						},
						Forward:         []forward.Forward{},
						Selector:        Selector{},
						EmptyImage:      true,
						ImagePullPolicy: v1.PullAlways,
						Image: &build.Info{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Interface: Localhost,
						PersistentVolumeInfo: &PersistentVolumeInfo{
							Enabled: true,
						},
						Push: &build.Info{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Secrets: make([]Secret, 0),
						Command: Command{Values: []string{"sh"}},
						Probes: &Probes{
							Liveness:  false,
							Readiness: false,
							Startup:   false,
						},
						Lifecycle: &Lifecycle{
							PostStart: false,
							PostStop:  false,
						},
						SecurityContext: &SecurityContext{
							RunAsUser:    pointer.Int64(0),
							RunAsGroup:   pointer.Int64(0),
							RunAsNonRoot: nil,
							FSGroup:      pointer.Int64(0),
						},
						SSHServerPort: 2222,
						Services:      []*Dev{},
						InitContainer: InitContainer{
							Image: OktetoBinImageTag,
						},
						Timeout: Timeout{
							Resources: 120 * time.Second,
							Default:   60 * time.Second,
						},
						Metadata: &Metadata{
							Labels:      Labels{},
							Annotations: Annotations{},
						},
						Environment: env.Environment{},
						Volumes:     []Volume{},
						Mode:        constants.OktetoSyncModeFieldValue,
					},
					"test-2": {
						Name: "test-2",
						Sync: Sync{
							RescanInterval: 300,
							Compression:    true,
							Folders: []SyncFolder{
								{
									LocalPath:  "app",
									RemotePath: "/app",
								},
							},
						},
						Forward:         []forward.Forward{},
						Selector:        Selector{},
						EmptyImage:      true,
						ImagePullPolicy: v1.PullAlways,
						Image: &build.Info{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Interface: Localhost,
						PersistentVolumeInfo: &PersistentVolumeInfo{
							Enabled: true,
						},
						Push: &build.Info{
							Name:       "",
							Context:    ".",
							Dockerfile: "Dockerfile",
							Target:     "",
						},
						Secrets: make([]Secret, 0),
						Command: Command{Values: []string{"sh"}},
						Probes: &Probes{
							Liveness:  false,
							Readiness: false,
							Startup:   false,
						},
						Lifecycle: &Lifecycle{
							PostStart: false,
							PostStop:  false,
						},
						SecurityContext: &SecurityContext{
							RunAsUser:    pointer.Int64(0),
							RunAsGroup:   pointer.Int64(0),
							RunAsNonRoot: nil,
							FSGroup:      pointer.Int64(0),
						},
						SSHServerPort: 2222,
						Services:      []*Dev{},
						InitContainer: InitContainer{
							Image: OktetoBinImageTag,
						},
						Timeout: Timeout{
							Resources: 120 * time.Second,
							Default:   60 * time.Second,
						},
						Metadata: &Metadata{
							Labels:      Labels{},
							Annotations: Annotations{},
						},
						Environment: env.Environment{},
						Volumes:     []Volume{},
						Mode:        constants.OktetoSyncModeFieldValue,
					},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "dev manifest with errors",
			manifest: []byte(`
dev:
  test-1:
    sync:
    - app:/app
    services:
    - name: svc
  test-2:
    sync:
    - app:/app
    services:
    - name: svc
sync:
- app:test
`),
			expected:        nil,
			isErrorExpected: true,
		},
		{
			name: "dev manifest with deploy",
			manifest: []byte(`
deploy:
  - okteto stack deploy
`),
			expected: &Manifest{
				Type:         OktetoManifestType,
				IsV2:         true,
				Dev:          map[string]*Dev{},
				Build:        map[string]*build.Info{},
				Dependencies: map[string]*deps.Dependency{},
				External:     externalresource.Section{},
				Destroy:      &DestroyInfo{},
				Deploy: &DeployInfo{
					Commands: []DeployCommand{
						{
							Name:    "okteto stack deploy",
							Command: "okteto stack deploy",
						},
					},
				},
			},
			isErrorExpected: false,
		},
		{
			name: "dev manifest with deploy",
			manifest: []byte(`
deploy:
  - okteto stack deploy
devs:
  - api
  - test
`),
			expected: &Manifest{
				Type:         OktetoManifestType,
				IsV2:         true,
				Dev:          map[string]*Dev{},
				Build:        map[string]*build.Info{},
				Dependencies: map[string]*deps.Dependency{},
				External:     externalresource.Section{},
				Destroy:      &DestroyInfo{},
				Deploy: &DeployInfo{
					Commands: []DeployCommand{
						{
							Name:    "okteto stack deploy",
							Command: "okteto stack deploy",
						},
					},
				},
			},
			isErrorExpected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest, err := Read(tt.manifest)
			if manifest != nil {
				for _, d := range manifest.Dev {
					d.parentSyncFolder = ""
				}
			}
			if err != nil && !tt.isErrorExpected {
				t.Fatalf("Not expecting error but got %s", err)
			} else if tt.isErrorExpected && err == nil {
				t.Fatal("Expected error but got none")
			}

			if err == nil && manifest != nil {
				manifest.Manifest = nil
			}

			if !assert.Equal(t, tt.expected, manifest) {

				t.Fatal("Failed")
			}
		})
	}
}

func TestManifestMarshalling(t *testing.T) {
	tests := []struct {
		name     string
		manifest *Manifest
		expected string
	}{
		{
			name: "destroy not empty",
			manifest: &Manifest{
				Destroy: &DestroyInfo{
					Commands: []DeployCommand{
						{
							Name:    "hello",
							Command: "hello",
						},
					},
				},
			},
			expected: "destroy:\n- hello\n",
		},
		{
			name:     "destroy empty",
			manifest: &Manifest{},
			expected: "{}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marshalled, err := yaml.Marshal(tt.manifest)
			if err != nil {
				t.Fatal(err)
			}

			if string(marshalled) != tt.expected {
				t.Errorf("didn't marshal correctly. Actual %s, Expected %s", marshalled, tt.expected)
			}
		})
	}
}

func TestDevModeUnmarshalling(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)

	tests := []struct {
		expected *Dev
		name     string
		input    []byte
	}{
		{
			name: "hybrid mode enabled",
			input: []byte(`mode: hybrid
selector:
  app.kubernetes.io/part-of: okteto
  app.kubernetes.io/component: frontend
command: ["sh", "-c", "yarn start"]
reverse:
  - 8080:8080`),
			expected: &Dev{
				Mode:    constants.OktetoHybridModeFieldValue,
				Workdir: wd,
				Selector: Selector{
					"app.kubernetes.io/part-of":   "okteto",
					"app.kubernetes.io/component": "frontend",
				},
				Command: Command{
					Values: []string{"sh", "-c", "yarn start"},
				},
				Reverse: []Reverse{
					{
						Remote: 8080,
						Local:  8080,
					},
				},
				Image: &build.Info{
					Name: "busybox",
				},
				Push:      &build.Info{},
				Secrets:   []Secret{},
				Probes:    &Probes{},
				Lifecycle: &Lifecycle{},
				Sync: Sync{
					Folders: []SyncFolder{},
				},
				Forward:     []forward.Forward{},
				Environment: env.Environment{},
				Volumes:     []Volume{},
				Services:    []*Dev{},
				Metadata: &Metadata{
					Labels:      Labels{},
					Annotations: Annotations{},
				},
				PersistentVolumeInfo: &PersistentVolumeInfo{
					Enabled: true,
				},
				InitContainer: InitContainer{
					Image: OktetoBinImageTag,
				},
			},
		},
		{
			name: "sync mode enabled",
			input: []byte(`mode: sync
selector:
  app.kubernetes.io/part-of: okteto
  app.kubernetes.io/component: api
image: okteto/golang:1
environment:
  - LOG_FORMATTER=text
command: sh
sync:
  - ./api:/usr/src/app
forward:
  - 2345:2345`),
			expected: &Dev{
				Mode: constants.OktetoSyncModeFieldValue,
				Selector: Selector{
					"app.kubernetes.io/part-of":   "okteto",
					"app.kubernetes.io/component": "api",
				},
				Command: Command{
					Values: []string{"sh"},
				},
				Image: &build.Info{
					Name: "okteto/golang:1",
				},
				Push:      &build.Info{},
				Secrets:   []Secret{},
				Probes:    &Probes{},
				Lifecycle: &Lifecycle{},
				Sync: Sync{
					Compression:    true,
					RescanInterval: 300,
					Folders: []SyncFolder{
						{
							LocalPath:  "./api",
							RemotePath: "/usr/src/app",
						},
					},
				},
				Forward: []forward.Forward{
					{
						Local:  2345,
						Remote: 2345,
					},
				},
				Environment: env.Environment{
					{
						Name:  "LOG_FORMATTER",
						Value: "text",
					},
				},
				Volumes:  []Volume{},
				Services: []*Dev{},
				Metadata: &Metadata{
					Labels:      Labels{},
					Annotations: Annotations{},
				},
				PersistentVolumeInfo: &PersistentVolumeInfo{
					Enabled: true,
				},
				InitContainer: InitContainer{
					Image: OktetoBinImageTag,
				},
			},
		},
		{
			name: "no mode, sync fallback",
			input: []byte(`
selector:
  app.kubernetes.io/part-of: okteto
  app.kubernetes.io/component: producer
image: okteto/golang:1
command: sh
sync:
  - ./producer:/usr/src/app
forward:
  - 2345:2345`),
			expected: &Dev{
				Mode: constants.OktetoSyncModeFieldValue,
				Selector: Selector{
					"app.kubernetes.io/part-of":   "okteto",
					"app.kubernetes.io/component": "producer",
				},
				Command: Command{
					Values: []string{"sh"},
				},
				Image: &build.Info{
					Name: "okteto/golang:1",
				},
				Push:      &build.Info{},
				Secrets:   []Secret{},
				Probes:    &Probes{},
				Lifecycle: &Lifecycle{},
				Sync: Sync{
					Compression:    true,
					RescanInterval: 300,
					Folders: []SyncFolder{
						{
							LocalPath:  "./producer",
							RemotePath: "/usr/src/app",
						},
					},
				},
				Forward: []forward.Forward{
					{
						Local:  2345,
						Remote: 2345,
					},
				},
				Environment: env.Environment{},
				Volumes:     []Volume{},
				Services:    []*Dev{},
				Metadata: &Metadata{
					Labels:      Labels{},
					Annotations: Annotations{},
				},
				PersistentVolumeInfo: &PersistentVolumeInfo{
					Enabled: true,
				},
				InitContainer: InitContainer{
					Image: OktetoBinImageTag,
				},
			},
		},
		{
			name: "hybrid mode with unsupported fields does not break",
			input: []byte(`mode: hybrid
selector:
  app.kubernetes.io/part-of: okteto
  app.kubernetes.io/component: producer
image: okteto/golang:1
command: sh
sync:
  - ./producer:/usr/src/app
forward:
  - 2345:2345`),
			expected: &Dev{
				Mode:    "hybrid",
				Workdir: wd,
				Selector: Selector{
					"app.kubernetes.io/part-of":   "okteto",
					"app.kubernetes.io/component": "producer",
				},
				Command: Command{
					Values: []string{"sh"},
				},
				Image: &build.Info{
					Name: "busybox",
				},
				Push:      &build.Info{},
				Secrets:   []Secret{},
				Probes:    &Probes{},
				Lifecycle: &Lifecycle{},
				Sync: Sync{
					Compression:    true,
					RescanInterval: 300,
					Folders: []SyncFolder{
						{
							LocalPath:  "./producer",
							RemotePath: "/usr/src/app",
						},
					},
				},
				Forward: []forward.Forward{
					{
						Local:  2345,
						Remote: 2345,
					},
				},
				Environment: env.Environment{},
				Volumes:     []Volume{},
				Services:    []*Dev{},
				Metadata: &Metadata{
					Labels:      Labels{},
					Annotations: Annotations{},
				},
				PersistentVolumeInfo: &PersistentVolumeInfo{
					Enabled: true,
				},
				InitContainer: InitContainer{
					Image: OktetoBinImageTag,
				},
			},
		},
		{
			name: "no valid mode return error",
			input: []byte(`
mode: invalid-mode
selector:
  app.kubernetes.io/part-of: okteto
  app.kubernetes.io/component: producer
image: okteto/golang:1
command: sh
sync:
  - ./producer:/usr/src/app
forward:
  - 2345:2345`),
			expected: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewDev()
			err := yaml.UnmarshalStrict(tt.input, result)
			if tt.expected != nil {
				assert.NoError(t, err)
				if !assert.Equal(t, tt.expected, result) {
					t.Fatal("Failed")
				}
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestWarnHybridUnsupportedFields(t *testing.T) {
	tests := []struct {
		name     string
		hybrid   *hybridModeInfo
		expected string
	}{
		{
			name: "All fields are supported",
			hybrid: &hybridModeInfo{
				Workdir: "/test",
				Mode:    "hybrid",
				Command: hybridCommand{
					Values: []string{"test"},
				},
			},
			expected: "",
		},
		{
			name: "All fields are unsupported",
			hybrid: &hybridModeInfo{
				UnsupportedFields: map[string]interface{}{
					"replicas":  2,
					"context":   "test",
					"namespace": "test",
				},
			},
			expected: "In hybrid mode, the field(s) 'context, namespace, replicas' specified in your manifest are ignored",
		},
		{
			name: "Some fields are unsupported",
			hybrid: &hybridModeInfo{
				Mode: "sync",
				Command: hybridCommand{
					Values: []string{"test"},
				},
				UnsupportedFields: map[string]interface{}{
					"context": "test",
				},
			},
			expected: "In hybrid mode, the field(s) 'context' specified in your manifest are ignored",
		},
		{
			name: "No fields",
			hybrid: &hybridModeInfo{
				UnsupportedFields: map[string]interface{}{},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := tt.hybrid.warnHybridUnsupportedFields()
			assert.Equal(t, tt.expected, output)
		})
	}
}

func TestDestroyInfoMarshalling(t *testing.T) {
	tests := []struct {
		name        string
		destroyInfo *DestroyInfo
		expected    string
	}{
		{
			name: "same-name-and-cmd",
			destroyInfo: &DestroyInfo{Commands: []DeployCommand{
				{
					Name:    "okteto build",
					Command: "okteto build",
				},
				{
					Name:    "okteto deploy",
					Command: "okteto deploy",
				},
			}},
			expected: "- okteto build\n- okteto deploy\n",
		},
		{
			name: "full",
			destroyInfo: &DestroyInfo{
				Image: "test",
				Commands: []DeployCommand{
					{
						Name:    "build",
						Command: "okteto build",
					},
					{
						Name:    "deploy",
						Command: "okteto deploy",
					},
				}},
			expected: "image: test\ncommands:\n- name: build\n  command: okteto build\n- name: deploy\n  command: okteto deploy\n",
		},
		{
			name: "different-name-cmd",
			destroyInfo: &DestroyInfo{Commands: []DeployCommand{
				{
					Name:    "build",
					Command: "okteto build",
				},
				{
					Name:    "deploy",
					Command: "okteto deploy",
				},
			}},
			expected: "commands:\n- name: build\n  command: okteto build\n- name: deploy\n  command: okteto deploy\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marshalled, err := yaml.Marshal(tt.destroyInfo)
			if err != nil {
				t.Fatal(err)
			}

			if string(marshalled) != tt.expected {
				t.Errorf("didn't marshal correctly. Actual %s, Expected %s", marshalled, tt.expected)
			}
		})
	}
}

func TestDestroyInfoUnmarshalling(t *testing.T) {
	tests := []struct {
		expected        *DestroyInfo
		name            string
		input           []byte
		isErrorExpected bool
	}{
		{
			name: "list of commands",
			input: []byte(`
- okteto stack deploy`),
			expected: &DestroyInfo{
				Commands: []DeployCommand{
					{
						Name:    "okteto stack deploy",
						Command: "okteto stack deploy",
					},
				},
			},
		},
		{
			name: "list of commands extended",
			input: []byte(`
- name: deploy stack
  command: okteto stack deploy`),
			expected: &DestroyInfo{
				Commands: []DeployCommand{
					{
						Name:    "deploy stack",
						Command: "okteto stack deploy",
					},
				},
			},
		},
		{
			name: "commands",
			input: []byte(`commands:
- okteto stack deploy`),
			expected: &DestroyInfo{
				Commands: []DeployCommand{
					{
						Name:    "okteto stack deploy",
						Command: "okteto stack deploy",
					},
				},
			},
		},
		{
			name: "compose with endpoints",
			input: []byte(`compose:
  manifest: path
  endpoints:
    - path: /
      service: app
      port: 80`),
			expected: &DestroyInfo{
				Commands: []DeployCommand{},
			},
			isErrorExpected: true,
		},
		{
			name: "all together",
			input: []byte(`commands:
- kubectl apply -f manifest.yml
compose:
  manifest: ./docker-compose.yml
  endpoints:
  - path: /
    service: frontend
    port: 80
  - path: /api
    service: api
    port: 8080`),
			expected: &DestroyInfo{
				Commands: []DeployCommand{},
			},
			isErrorExpected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewDestroyInfo()

			err := yaml.UnmarshalStrict(tt.input, &result)
			if err != nil && !tt.isErrorExpected {
				t.Fatalf("Not expecting error but got %s", err)
			} else if tt.isErrorExpected && err == nil {
				t.Fatal("Expected error but got none")
			}

			if !assert.Equal(t, tt.expected, result) {
				t.Fatal("Failed")
			}
		})
	}
}

func TestDeployInfoUnmarshalling(t *testing.T) {
	tests := []struct {
		expected           *DeployInfo
		name               string
		deployInfoManifest []byte
		isErrorExpected    bool
	}{
		{
			name: "list of commands",
			deployInfoManifest: []byte(`
- okteto stack deploy`),
			expected: &DeployInfo{
				Commands: []DeployCommand{
					{
						Name:    "okteto stack deploy",
						Command: "okteto stack deploy",
					},
				},
			},
		},
		{
			name: "list of commands extended",
			deployInfoManifest: []byte(`
- name: deploy stack
  command: okteto stack deploy`),
			expected: &DeployInfo{
				Commands: []DeployCommand{
					{
						Name:    "deploy stack",
						Command: "okteto stack deploy",
					},
				},
			},
		},
		{
			name: "commands",
			deployInfoManifest: []byte(`commands:
- okteto stack deploy`),
			expected: &DeployInfo{
				Commands: []DeployCommand{
					{
						Name:    "okteto stack deploy",
						Command: "okteto stack deploy",
					},
				},
			},
		},
		{
			name: "compose with endpoints",
			deployInfoManifest: []byte(`compose:
  manifest: path
  endpoints:
    - path: /
      service: app
      port: 80`),
			expected: &DeployInfo{
				Commands: []DeployCommand{},
			},
			isErrorExpected: true,
		},
		{
			name: "all together",
			deployInfoManifest: []byte(`commands:
- kubectl apply -f manifest.yml
compose:
  manifest: ./docker-compose.yml
  endpoints:
  - path: /
    service: frontend
    port: 80
  - path: /api
    service: api
    port: 8080`),
			expected: &DeployInfo{
				Commands: []DeployCommand{},
			},
			isErrorExpected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewDeployInfo()

			err := yaml.UnmarshalStrict(tt.deployInfoManifest, &result)
			if err != nil && !tt.isErrorExpected {
				t.Fatalf("Not expecting error but got %s", err)
			} else if tt.isErrorExpected && err == nil {
				t.Fatal("Expected error but got none")
			}

			if !assert.Equal(t, tt.expected, result) {
				t.Fatal("Failed")
			}
		})
	}
}

func TestDeployInfoMarshalling(t *testing.T) {
	tests := []struct {
		name       string
		deployInfo *DeployInfo
		expected   string
	}{
		{
			name: "same-name-and-cmd",
			deployInfo: &DeployInfo{Commands: []DeployCommand{
				{
					Name:    "okteto build",
					Command: "okteto build",
				},
				{
					Name:    "okteto deploy",
					Command: "okteto deploy",
				},
			}},
			expected: "- okteto build\n- okteto deploy\n",
		},
		{
			name: "different-name-cmd",
			deployInfo: &DeployInfo{Commands: []DeployCommand{
				{
					Name:    "build",
					Command: "okteto build",
				},
				{
					Name:    "deploy",
					Command: "okteto deploy",
				},
			}},
			expected: "commands:\n- name: build\n  command: okteto build\n- name: deploy\n  command: okteto deploy\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marshalled, err := yaml.Marshal(tt.deployInfo)
			if err != nil {
				t.Fatal(err)
			}

			if string(marshalled) != tt.expected {
				t.Errorf("didn't marshal correctly. Actual %s, Expected %s", marshalled, tt.expected)
			}
		})
	}
}

func TestComposeSectionInfoUnmarshalling(t *testing.T) {
	tests := []struct {
		expected            *ComposeSectionInfo
		name                string
		composeInfoManifest []byte
	}{
		{
			name: "list of compose",
			composeInfoManifest: []byte(`- docker-compose.yml
- docker-compose.dev.yml`),
			expected: &ComposeSectionInfo{
				ComposesInfo: []ComposeInfo{
					{
						File: "docker-compose.yml",
					},
					{
						File: "docker-compose.dev.yml",
					},
				},
			},
		},
		{
			name:                "a docker compose",
			composeInfoManifest: []byte(`docker-compose.yml`),
			expected: &ComposeSectionInfo{
				ComposesInfo: []ComposeInfo{
					{
						File: "docker-compose.yml",
					},
				},
			},
		},
		{
			name:                "extended notation one compose",
			composeInfoManifest: []byte(`manifest: docker-compose.yml`),
			expected: &ComposeSectionInfo{
				ComposesInfo: []ComposeInfo{
					{
						File: "docker-compose.yml",
					},
				},
			},
		},
		{
			name: "multiple compose under `manifest`",
			composeInfoManifest: []byte(`
manifest:
  - docker-compose.yml
  - docker-compose.dev.yml`),
			expected: &ComposeSectionInfo{
				ComposesInfo: []ComposeInfo{
					{
						File: "docker-compose.yml",
					},
					{
						File: "docker-compose.dev.yml",
					},
				},
			},
		},
		{
			name: "compose with services",
			composeInfoManifest: []byte(`
file: docker-compose.yml
services:
  - a
  - b`),
			expected: &ComposeSectionInfo{
				ComposesInfo: []ComposeInfo{
					{
						File:             "docker-compose.yml",
						ServicesToDeploy: []string{"a", "b"},
					},
				},
			},
		},
		{
			name: "multiple compose with services",
			composeInfoManifest: []byte(`
- file: docker-compose.yml
  services:
    - a
    - b
- file: another-docker-compose.yml
  services: c`),
			expected: &ComposeSectionInfo{
				ComposesInfo: []ComposeInfo{
					{
						File:             "docker-compose.yml",
						ServicesToDeploy: []string{"a", "b"},
					},
					{
						File:             "another-docker-compose.yml",
						ServicesToDeploy: []string{"c"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ComposeSectionInfo{}

			err := yaml.UnmarshalStrict(tt.composeInfoManifest, &result)
			if err != nil {
				t.Fatalf("Not expecting error but got %s", err)
			}

			if !assert.Equal(t, tt.expected, result) {
				t.Fatal("Failed")
			}
		})
	}
}

func TestComposeInfoUnmarshalling(t *testing.T) {
	tests := []struct {
		expected             *ComposeInfo
		name                 string
		manifestListManifest []byte
	}{
		{
			name: "docker compose without key",
			manifestListManifest: []byte(`
docker-compose.yml`),
			expected: &ComposeInfo{
				File: "docker-compose.yml",
			},
		},
		{
			name: "docker compose",
			manifestListManifest: []byte(`
file: docker-compose.yml`),
			expected: &ComposeInfo{
				File: "docker-compose.yml",
			},
		},
		{
			name: "docker compose with services",
			manifestListManifest: []byte(`
file: docker-compose.yml
services: a`),
			expected: &ComposeInfo{
				File: "docker-compose.yml",
				ServicesToDeploy: []string{
					"a",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ComposeInfo{}

			err := yaml.UnmarshalStrict(tt.manifestListManifest, &result)
			if err != nil {
				t.Fatalf("Not expecting error but got %s", err)
			}

			if !assert.Equal(t, tt.expected, result) {
				t.Fatal("Failed")
			}
		})
	}
}

func TestManifestBuildUnmarshalling(t *testing.T) {
	tests := []struct {
		expected        build.ManifestBuild
		name            string
		buildManifest   []byte
		isErrorExpected bool
	}{
		{
			name:          "unmarshalling-relative-path",
			buildManifest: []byte(`service1: ./service1`),
			expected: build.ManifestBuild{
				"service1": {
					Name:    "./service1",
					Context: "",
				},
			},
		},
		{
			name: "unmarshalling-all-fields",
			buildManifest: []byte(`service2:
  image: image-tag
  context: ./service2
  dockerfile: Dockerfile
  args:
    key1: value1
  cache_from:
    - cache-image
  secrets:
    mysecret: source
    othersecret: othersource`),
			expected: build.ManifestBuild{
				"service2": {
					Context:    "./service2",
					Dockerfile: "Dockerfile",
					Image:      "image-tag",
					Args: build.Args{
						{
							Name:  "key1",
							Value: "value1",
						},
					},
					CacheFrom: []string{"cache-image"},
					Secrets: build.Secrets{
						"mysecret":    "source",
						"othersecret": "othersource",
					},
				},
			},
		},
		{
			name: "invalid-fields",
			buildManifest: []byte(`service1:
  file: Dockerfile`),
			expected:        build.ManifestBuild{},
			isErrorExpected: true,
		},
		{
			name: "cache_from-supports-str",
			buildManifest: []byte(`service3:
  cache_from: cache-image`),
			expected: build.ManifestBuild{
				"service3": {
					CacheFrom: []string{"cache-image"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result build.ManifestBuild
			err := yaml.UnmarshalStrict(tt.buildManifest, &result)
			if err != nil && !tt.isErrorExpected {
				t.Fatalf("Not expecting error but got %s", err)
			} else if tt.isErrorExpected && err == nil {
				t.Fatal("Expected error but got none")
			}

			if !assert.Equal(t, tt.expected, result) {
				t.Fatal("Failed")
			}
		})
	}
}

func TestBuildDependsOnUnmarshalling(t *testing.T) {
	tests := []struct {
		name          string
		buildManifest []byte
		expected      build.DependsOn
	}{
		{
			name:          "single string",
			buildManifest: []byte(`a`),
			expected:      build.DependsOn{"a"},
		},
		{
			name: "list",
			buildManifest: []byte(`- a
- b`),
			expected: build.DependsOn{"a", "b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result build.DependsOn
			err := yaml.UnmarshalStrict(tt.buildManifest, &result)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDependencyUnmashalling(t *testing.T) {
	tests := []struct {
		expected *deps.Dependency
		name     string
		data     []byte
	}{
		{
			name: "single line",
			data: []byte(`https://github/test`),
			expected: &deps.Dependency{
				Repository: "https://github/test",
			},
		},
		{
			name: "repository and branch",
			data: []byte(`repository: https://github/test
branch: main`),
			expected: &deps.Dependency{
				Repository: "https://github/test",
				Branch:     "main",
			},
		},
		{
			name: "repository,branch and manifest",
			data: []byte(`repository: https://github/test
branch: main
manifest: okteto.yml`),
			expected: &deps.Dependency{
				Repository:   "https://github/test",
				Branch:       "main",
				ManifestPath: "okteto.yml",
			},
		},
		{
			name: "repository,branch and manifest and variables",
			data: []byte(`repository: https://github/test
branch: main
manifest: okteto.yml
variables:
  key: value`),
			expected: &deps.Dependency{
				Repository:   "https://github/test",
				Branch:       "main",
				ManifestPath: "okteto.yml",
				Variables: env.Environment{
					env.Var{
						Name:  "key",
						Value: "value",
					},
				},
			},
		},
		{
			name: "repository,branch,manifest,variables and wait",
			data: []byte(`repository: https://github/test
branch: main
manifest: okteto.yml
variables:
  key: value
wait: true`),
			expected: &deps.Dependency{
				Repository:   "https://github/test",
				Branch:       "main",
				ManifestPath: "okteto.yml",
				Wait:         true,
				Variables: env.Environment{
					env.Var{
						Name:  "key",
						Value: "value",
					},
				},
			},
		},
		{
			name: "repository,branch,manifest,variables,wait and timeout",
			data: []byte(`repository: https://github/test
branch: main
manifest: okteto.yml
variables:
  key: value
wait: true
timeout: 15m`),
			expected: &deps.Dependency{
				Repository:   "https://github/test",
				Branch:       "main",
				ManifestPath: "okteto.yml",
				Wait:         true,
				Variables: env.Environment{
					env.Var{
						Name:  "key",
						Value: "value",
					},
				},
				Timeout: 15 * time.Minute,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result *deps.Dependency

			if err := yaml.UnmarshalStrict(tt.data, &result); err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, tt.expected, result)
		})
	}
}
