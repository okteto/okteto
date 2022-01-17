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
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/model/build"
	"github.com/okteto/okteto/pkg/model/constants"
	"github.com/okteto/okteto/pkg/model/environment"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
)

func Test_loadSelector(t *testing.T) {
	tests := []struct {
		name     string
		selector Selector
		value    string
		want     Selector
	}{
		{
			name:     "no-var",
			selector: Selector{"a": "1", "b": "2"},
			value:    "3",
			want:     Selector{"a": "1", "b": "2"},
		},
		{
			name:     "var",
			selector: Selector{"a": "1", "b": "${value}"},
			value:    "3",
			want:     Selector{"a": "1", "b": "3"},
		},
		{
			name:     "missing",
			selector: Selector{"a": "1", "b": "${valueX}"},
			value:    "1",
			want:     Selector{"a": "1", "b": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev := &Dev{Selector: tt.selector}
			os.Setenv("value", tt.value)
			if err := dev.loadSelector(); err != nil {
				t.Fatalf("couldn't load selector")
			}

			for key, value := range dev.Labels {
				if tt.want[key] != value {
					t.Errorf("got: '%v', expected: '%v'", dev.Labels, tt.want)
				}
			}
		})
	}
}

func TestDev_validateName(t *testing.T) {
	tests := []struct {
		name    string
		devName string
		wantErr bool
	}{
		{name: "empty", devName: "", wantErr: true},
		{name: "starts-with-dash", devName: "-bad-name", wantErr: true},
		{name: "ends-with-dash", devName: "bad-name-", wantErr: true},
		{name: "symbols", devName: "1$good-2", wantErr: true},
		{name: "alphanumeric", devName: "good-2", wantErr: false},
		{name: "good", devName: "good-name", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dev := &Dev{
				Name:            tt.devName,
				ImagePullPolicy: apiv1.PullAlways,
				Image:           &build.Build{},
				Push:            &build.Build{},
				Sync: Sync{
					Folders: []SyncFolder{
						{
							LocalPath:  ".",
							RemotePath: "/app",
						},
					},
				},
			}
			// Since dev isn't being unmarshalled through Read, apply defaults
			// before validating.
			if err := dev.SetDefaults(); err != nil {
				t.Fatalf("error applying defaults: %v", err)
			}
			if err := dev.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Dev.validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetTimeout(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		want    time.Duration
		wantErr bool
	}{
		{name: "default value", want: 60 * time.Second},
		{name: "env var", want: 134 * time.Second, env: "134s"},
		{name: "bad env var", wantErr: true, env: "bad value"},
	}

	original := os.Getenv(constants.OktetoTimeoutEnvVar)
	defer os.Setenv(constants.OktetoTimeoutEnvVar, original)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				os.Setenv(constants.OktetoTimeoutEnvVar, tt.env)
			}
			got, err := GetTimeout()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTimeout() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("GetTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_expandEnvFiles(t *testing.T) {
	file, err := os.CreateTemp("", ".env")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	tests := []struct {
		name     string
		dev      *Dev
		envs     []byte
		expected environment.Environment
	}{
		{
			name: "add new envs",
			dev: &Dev{
				Environment: environment.Environment{},
				EnvFiles: environment.EnvFiles{
					file.Name(),
				},
			},
			envs: []byte("key1=value1"),
			expected: environment.Environment{
				environment.EnvVar{
					Name:  "key1",
					Value: "value1",
				},
			},
		},
		{
			name: "dont overwrite envs",
			dev: &Dev{
				Environment: environment.Environment{
					{
						Name:  "key1",
						Value: "value1",
					},
				},
				EnvFiles: environment.EnvFiles{
					file.Name(),
				},
			},
			envs: []byte("key1=value100"),
			expected: environment.Environment{
				environment.EnvVar{
					Name:  "key1",
					Value: "value1",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s is present", tt.name), func(t *testing.T) {
			if _, err = file.Write(tt.envs); err != nil {
				t.Fatal("Failed to write to temporary file", err)
			}
			if err := tt.dev.ExpandEnvFiles(); err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, tt.expected, tt.dev.Environment)
		})
	}
}
