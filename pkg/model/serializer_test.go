// Copyright 2020 The Okteto Authors
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
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strings"
	"testing"

	yaml "gopkg.in/yaml.v2"
)

func TestReverseMashalling(t *testing.T) {
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

func TestEnvVarMashalling(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected EnvVar
	}{
		{
			"key-value",
			[]byte(`env=production`),
			EnvVar{Name: "env", Value: "production"},
		},
		{
			"key-value-complex",
			[]byte(`env='production=11231231asa#$˜GADAFA'`),
			EnvVar{Name: "env", Value: "'production=11231231asa#$˜GADAFA'"},
		},
		{
			"key-value-with-env-var",
			[]byte(`env=$DEV_ENV`),
			EnvVar{Name: "env", Value: "test_environment"},
		},
		{
			"key-value-with-env-var-in-string",
			[]byte(`env=my_env;$DEV_ENV;prod`),
			EnvVar{Name: "env", Value: "my_env;test_environment;prod"},
		},
		{
			"simple-key",
			[]byte(`noenv`),
			EnvVar{Name: "noenv", Value: ""},
		},
		{
			"key-with-no-value",
			[]byte(`noenv=`),
			EnvVar{Name: "noenv", Value: ""},
		},
		{
			"key-with-env-var-not-defined",
			[]byte(`noenv=$UNDEFINED`),
			EnvVar{Name: "noenv", Value: ""},
		},
		{
			"just-env-var",
			[]byte(`$DEV_ENV`),
			EnvVar{Name: "test_environment", Value: ""},
		},
		{
			"just-env-var-undefined",
			[]byte(`$UNDEFINED`),
			EnvVar{Name: "", Value: ""},
		},
		{
			"local_env_expanded",
			[]byte(`OKTETO_TEST_ENV_MARSHALLING`),
			EnvVar{Name: "OKTETO_TEST_ENV_MARSHALLING", Value: "true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var result EnvVar
			if err := os.Setenv("DEV_ENV", "test_environment"); err != nil {
				t.Fatal(err)
			}

			if err := os.Setenv("OKTETO_TEST_ENV_MARSHALLING", "true"); err != nil {
				t.Fatal(err)
			}

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

func TestCommandUnmashalling(t *testing.T) {
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

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("didn't unmarshal correctly. Actual %+v, Expected %+v", result, tt.expected)
			}
		})
	}
}

func TestCommandMashalling(t *testing.T) {
	tests := []struct {
		name     string
		command  Command
		expected string
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

func TestImageMashalling(t *testing.T) {
	tests := []struct {
		name     string
		image    BuildInfo
		expected string
	}{
		{
			name:     "single-name",
			image:    BuildInfo{Name: "image-name"},
			expected: "image-name\n",
		},
		{
			name:     "single-name-and-defaults",
			image:    BuildInfo{Name: "image-name", Context: "."},
			expected: "image-name\n",
		},
		{
			name:     "build",
			image:    BuildInfo{Name: "image-name", Context: "path"},
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

func TestHealthcheckMashalling(t *testing.T) {
	tests := []struct {
		name         string
		healthchecks Probes
		expected     string
	}{
		{
			name:         "liveness-true-and-defaults",
			healthchecks: Probes{Liveness: true},
			expected:     "liveness: true\n",
		},
		{
			name:         "all-healthchecks-true",
			healthchecks: Probes{Liveness: true, Readiness: true, Startup: true},
			expected:     "true\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			marshalled, err := yaml.Marshal(tt.healthchecks)
			if err != nil {
				t.Fatal(err)
			}

			if string(marshalled) != tt.expected {
				t.Errorf("didn't marshal correctly. Actual %s, Expected %s", marshalled, tt.expected)
			}
		})
	}
}

func TestSecretMashalling(t *testing.T) {
	file, err := ioutil.TempFile("/tmp", "okteto-secret-test")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(file.Name())

	if err := os.Setenv("TEST_HOME", file.Name()); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		data          string
		expected      *Secret
		expectedError bool
	}{
		{
			"local:remote",
			fmt.Sprintf("%s:/remote", file.Name()),
			&Secret{LocalPath: file.Name(), RemotePath: "/remote", Mode: 420},
			false,
		},
		{
			"local:remote:mode",
			fmt.Sprintf("%s:/remote:400", file.Name()),
			&Secret{LocalPath: file.Name(), RemotePath: "/remote", Mode: 256},
			false,
		},
		{
			"variables",
			"$TEST_HOME:/remote",
			&Secret{LocalPath: file.Name(), RemotePath: "/remote", Mode: 420},
			false,
		},
		{
			"too-short",
			"local",
			nil,
			true,
		},
		{
			"too-long",
			"local:remote:mode:other",
			nil,
			true,
		},
		{
			"wrong-local",
			"/local:/remote:400",
			nil,
			true,
		},
		{
			"wrong-remote",
			fmt.Sprintf("%s:remote", file.Name()),
			nil,
			true,
		},
		{
			"wrong-mode",
			fmt.Sprintf("%s:/remote:aaa", file.Name()),
			nil,
			true,
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

func TestVolumeMashalling(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected Volume
	}{
		{
			"global",
			[]byte("/path"),
			Volume{LocalPath: "", RemotePath: "/path"},
		},
		{
			"relative",
			[]byte("sub:/path"),
			Volume{LocalPath: "sub", RemotePath: "/path"},
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
		dev      Dev
		expected string
	}{
		{
			name:     "healtcheck-not-defaults",
			dev:      Dev{Name: "name-test", Probes: &Probes{Liveness: true}},
			expected: "name: name-test\nprobes:\n  liveness: true\n",
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
			marshalled, err := yaml.Marshal(tt.dev)
			if err != nil {
				t.Fatal(err)
			}

			if string(marshalled) != tt.expected {
				t.Errorf("didn't marshal correctly. Actual %s, Expected %s", marshalled, tt.expected)
			}
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
			},
		},
		{
			name: "full-endpoint",
			data: []byte("labels:\n  key1: value1\nannotations:\n  key2: value2\nrules:\n- path: /\n  service: test\n  port: 8080"),
			expected: Endpoint{
				Labels:      map[string]string{"key1": "value1"},
				Annotations: map[string]string{"key2": "value2"},
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
