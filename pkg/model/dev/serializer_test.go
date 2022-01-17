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

package dev

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/model/constants"
	yaml "gopkg.in/yaml.v2"
)

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
			marshalled, err := yaml.Marshal(&tt.dev)
			if err != nil {
				t.Fatal(err)
			}

			if string(marshalled) != tt.expected {
				t.Errorf("didn't marshal correctly. Actual %s, Expected %s", marshalled, tt.expected)
			}
		})
	}
}

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

func TestProbesMashalling(t *testing.T) {
	tests := []struct {
		name     string
		probes   Probes
		expected string
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

func TestLifecycleMashalling(t *testing.T) {
	tests := []struct {
		name      string
		lifecycle Lifecycle
		expected  string
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

func TestDurationUnmashalling(t *testing.T) {
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

func TestTimeoutUnmashalling(t *testing.T) {
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

func TestSyncUnmashalling(t *testing.T) {
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
				RescanInterval: constants.DefaultSyncthingRescanInterval,
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

func TestSyncFoldersUnmashalling(t *testing.T) {
	os.Setenv("REMOTE_PATH", "/usr/src/app")
	tests := []struct {
		name     string
		data     []byte
		expected SyncFolder
	}{
		{
			name:     "same dir",
			data:     []byte(`.:/usr/src/app`),
			expected: SyncFolder{LocalPath: ".", RemotePath: "/usr/src/app"},
		},
		{
			name:     "same dir with env var",
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
