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

package exec

import (
	"context"
	"testing"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNewOptions(t *testing.T) {
	testCases := []struct {
		expected      *options
		expectedError error
		name          string
		argsIn        []string
		argsLenAtDash int
	}{
		{
			name:          "Empty args",
			argsIn:        []string{},
			argsLenAtDash: 0,
			expected:      nil,
			expectedError: errCommandRequired,
		},
		{
			name:          "Args with dev name",
			argsIn:        []string{"dev1"},
			argsLenAtDash: -1,
			expected:      nil,
			expectedError: errCommandRequired,
		},
		{
			name:          "Args with command",
			argsIn:        []string{"echo", "test"},
			argsLenAtDash: 0,
			expected: &options{
				command:     []string{"echo", "test"},
				devSelector: utils.NewOktetoSelector("Select which development container to exec:", "Development container"),
			},
		},
		{
			name:          "Args with dev name and command",
			argsIn:        []string{"dev1", "echo", "test"},
			argsLenAtDash: 1,
			expected: &options{
				devName:           "dev1",
				firstArgIsDevName: true,
				command:           []string{"echo", "test"},
				devSelector:       utils.NewOktetoSelector("Select which development container to exec:", "Development container"),
			},
		},
		{
			name:          "Args with dev name no command",
			argsIn:        []string{"dev1"},
			argsLenAtDash: -1,
			expected:      nil,
			expectedError: errCommandRequired,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := newOptions(tc.argsIn, tc.argsLenAtDash)
			assert.Equal(t, tc.expected, opts)
			assert.ErrorIs(t, err, tc.expectedError)
		})
	}
}

type fakeDevSelector struct {
	err     error
	devName string
}

func (f *fakeDevSelector) AskForOptionsOkteto([]utils.SelectorItem, int) (string, error) {
	return f.devName, f.err
}
func TestSetDevFromManifest(t *testing.T) {
	ioControl := io.NewIOController()
	// Define test cases using a slice of structs

	objects := []runtime.Object{
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dev1",
				Namespace: "ns",
				Labels: map[string]string{
					constants.DevLabel: "true",
				},
			},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dev2",
				Namespace: "ns",
				Labels: map[string]string{
					constants.DevLabel: "true",
				},
			},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dev3",
				Namespace: "ns",
			},
		},
	}
	provider := test.NewFakeK8sProvider(objects...)
	ctx := context.Background()
	testCases := []struct {
		expectedError error
		options       *options
		devs          model.ManifestDevs
		name          string
		expectedDev   string
	}{
		{
			name: "Dev name already set",
			options: &options{
				devName: "dev1",
			},
			devs:          model.ManifestDevs{},
			expectedDev:   "dev1",
			expectedError: nil,
		},
		{
			name: "Select dev from manifest",
			options: &options{
				devName: "",
				devSelector: &fakeDevSelector{
					devName: "dev1",
					err:     nil,
				},
			},
			devs: model.ManifestDevs{
				"dev1": &model.Dev{
					Name: "dev1",
				},
				"dev2": &model.Dev{
					Name: "dev1",
				},
			},
			expectedDev:   "dev1",
			expectedError: nil,
		},
		{
			name: "no dev in dev mode",
			options: &options{
				devName: "",
				devSelector: &fakeDevSelector{
					devName: "dev1",
					err:     nil,
				},
			},
			devs: model.ManifestDevs{
				"dev3": &model.Dev{
					Name: "dev3",
				},
			},
			expectedDev:   "",
			expectedError: errNoDevContainerInDevMode,
		},
		{
			name: "Failed to select dev",
			options: &options{
				devName: "",
				devSelector: &fakeDevSelector{
					devName: "",
					err:     assert.AnError,
				},
			},
			devs: model.ManifestDevs{
				"dev1": &model.Dev{
					Name: "dev1",
				},
				"dev2": &model.Dev{
					Name: "dev1",
				},
			},
			expectedDev:   "",
			expectedError: assert.AnError,
		},
	}

	// Loop through test cases and run assertions
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.options.setDevFromManifest(ctx, tc.devs, "ns", provider, ioControl)
			assert.Equal(t, tc.expectedDev, tc.options.devName)
			assert.ErrorIs(t, err, tc.expectedError)
		})
	}
}

func TestValidate(t *testing.T) {
	// Define test cases using a slice of structs
	testCases := []struct {
		expected error
		options  *options
		devs     model.ManifestDevs
		name     string
	}{
		{
			name: "Missing dev name",
			options: &options{
				command: []string{"echo", "test"},
			},
			devs:     model.ManifestDevs{},
			expected: errDevNameRequired,
		},
		{
			name: "Invalid dev name (not defined)",
			options: &options{
				devName: "dev2",
				command: []string{"echo", "test"},
			},
			devs:     model.ManifestDevs{},
			expected: &errDevNotInManifest{devName: "dev2"},
		},
		{
			name: "Valid options",
			options: &options{
				devName: "dev1",
				command: []string{"echo", "test"},
			},
			devs: model.ManifestDevs{
				"dev1": &model.Dev{},
			},
			expected: nil,
		},
	}

	// Loop through test cases and run assertions
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.options.validate(tc.devs)
			if tc.expected != nil {
				assert.ErrorContains(t, err, tc.expected.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
