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

package args

import (
	"context"
	"testing"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNewDevCommandArgParser(t *testing.T) {
	lister := NewDevModeOnLister(nil)
	ioControl := io.NewIOController()

	parser := NewDevCommandArgParser(lister, ioControl, true)

	assert.NotNil(t, parser)
	assert.Equal(t, lister, parser.devLister)
	assert.Equal(t, ioControl, parser.ioCtrl)
	assert.Equal(t, true, parser.checkIfCmdIsEmpty)
}

func TestParseFromArgs(t *testing.T) {
	testCases := []struct {
		expected      *Result
		name          string
		argsIn        []string
		argsLenAtDash int
	}{
		{
			name:          "Args with command",
			argsIn:        []string{"echo", "test"},
			argsLenAtDash: 0,
			expected: &Result{
				Command: []string{"echo", "test"},
			},
		},
		{
			name:          "Args with dev name and command",
			argsIn:        []string{"dev1", "echo", "test"},
			argsLenAtDash: 1,
			expected: &Result{
				DevName:           "dev1",
				FirstArgIsDevName: true,
				Command:           []string{"echo", "test"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := &DevCommandArgParser{}
			result := parser.parseFromArgs(tc.argsIn, tc.argsLenAtDash)
			assert.Equal(t, tc.expected, result)
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
		options       *Result
		selector      devSelector
		devs          model.ManifestDevs
		name          string
		expectedDev   string
	}{
		{
			name: "Dev name already set",
			options: &Result{
				DevName: "dev1",
			},
			devs:          model.ManifestDevs{},
			expectedDev:   "dev1",
			expectedError: nil,
		},
		{
			name: "Select dev from manifest",
			options: &Result{
				DevName: "",
			},
			selector: &fakeDevSelector{
				devName: "dev1",
				err:     nil,
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
			options: &Result{
				DevName: "",
			},
			selector: &fakeDevSelector{
				devName: "dev1",
				err:     nil,
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
			options: &Result{
				DevName: "",
			},
			selector: &fakeDevSelector{
				devName: "",
				err:     assert.AnError,
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

	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"ns": {
				Namespace: "ns",
			},
		},
		CurrentContext: "ns",
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := &DevCommandArgParser{
				devSelector: tc.selector,
				ioCtrl:      ioControl,
				devLister:   NewDevModeOnLister(provider),
			}
			result, err := parser.setDevNameFromManifest(ctx, tc.options, tc.devs, "ns")
			if result != nil {
				assert.Equal(t, tc.expectedDev, result.DevName)
				assert.ErrorIs(t, err, tc.expectedError)
			} else {
				assert.ErrorIs(t, err, tc.expectedError)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	testCases := []struct {
		expected error
		options  *Result
		devs     model.ManifestDevs
		name     string
	}{
		{
			name: "Missing dev name",
			options: &Result{
				Command: []string{"echo", "test"},
			},
			devs:     model.ManifestDevs{},
			expected: errDevNameRequired,
		},
		{
			name: "Invalid dev name (not defined)",
			options: &Result{
				DevName: "dev2",
				Command: []string{"echo", "test"},
			},
			devs:     model.ManifestDevs{},
			expected: &errDevNotInManifest{devName: "dev2"},
		},
		{
			name: "Valid options",
			options: &Result{
				DevName: "dev1",
				Command: []string{"echo", "test"},
			},
			devs: model.ManifestDevs{
				"dev1": &model.Dev{},
			},
			expected: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := &DevCommandArgParser{}
			err := parser.validate(tc.options, tc.devs)
			if tc.expected != nil {
				assert.ErrorContains(t, err, tc.expected.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
