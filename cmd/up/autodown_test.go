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

package up

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/client-go/kubernetes/fake"
)

type mockAnalyticsTracker struct {
	mock.Mock
}

func (m *mockAnalyticsTracker) TrackEvent(event string, properties map[string]interface{}) {
	m.Called(event, properties)
}

func (m *mockAnalyticsTracker) TrackDeploy(metadata analytics.DeployMetadata) {
	m.Called(metadata)
}

func (m *mockAnalyticsTracker) TrackUp(metadata *analytics.UpMetricsMetadata) {
	m.Called(metadata)
}

func (m *mockAnalyticsTracker) TrackDown(success bool) {
	m.Called(success)
}

func (m *mockAnalyticsTracker) TrackDownVolumes(success bool) {
	m.Called(success)
}

func (m *mockAnalyticsTracker) TrackImageBuild(ctx context.Context, meta *analytics.ImageBuildMetadata) {
	m.Called(ctx, meta)
}

type mockDownCmdRunner struct {
	mock.Mock
}

func (m *mockDownCmdRunner) Run(app apps.App, dev *model.Dev, namespace string, trMap map[string]*apps.Translation, wait bool) error {
	args := m.Called(app, dev, namespace, trMap, wait)
	return args.Error(0)
}

func TestNewAutoDown(t *testing.T) {
	tests := []struct {
		name           string
		envValue       string
		expectedResult bool
	}{
		{
			name:           "AutoDown disabled by default",
			envValue:       "",
			expectedResult: false,
		},
		{
			name:           "AutoDown enabled",
			envValue:       "true",
			expectedResult: true,
		},
		{
			name:           "AutoDown disabled explicitly",
			envValue:       "false",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.envValue != "" {
				t.Setenv(autoDownEnvVar, tt.envValue)
			}

			ioCtrl := io.NewIOController()
			k8sLogger := io.NewK8sLogger()
			at := &mockAnalyticsTracker{}

			ad := newAutoDown(ioCtrl, k8sLogger, at, analytics.NewUpMetricsMetadata())

			assert.Equal(t, tt.expectedResult, ad.autoDown)
			assert.NotNil(t, ad.ioCtrl)
			assert.NotNil(t, ad.k8sLogger)
			assert.NotNil(t, ad.analyticsTracker)
		})
	}
}

func TestAutoDownRunner_Run(t *testing.T) {
	tests := []struct {
		name          string
		autoDown      bool
		dev           *model.Dev
		namespace     string
		mockSetup     func(*mockAnalyticsTracker, *mockDownCmdRunner)
		expectedError bool
	}{
		{
			name:      "AutoDown disabled",
			autoDown:  false,
			dev:       &model.Dev{},
			namespace: "test-namespace",
			mockSetup: func(at *mockAnalyticsTracker, downCmd *mockDownCmdRunner) {
				// No expectations needed as it should return early
			},
			expectedError: false,
		},
		{
			name:      "AutoDown enabled with sandbox deployment",
			autoDown:  true,
			dev:       &model.Dev{Autocreate: true},
			namespace: "test-namespace",
			mockSetup: func(at *mockAnalyticsTracker, downCmd *mockDownCmdRunner) {
				downCmd.On("Run", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			ioCtrl := io.NewIOController()
			k8sLogger := io.NewK8sLogger()
			at := &mockAnalyticsTracker{}
			downCmd := &mockDownCmdRunner{}

			tt.mockSetup(at, downCmd)

			ad := &autoDownRunner{
				autoDown:         tt.autoDown,
				ioCtrl:           ioCtrl,
				k8sLogger:        k8sLogger,
				analyticsTracker: at,
				downCmd:          downCmd,
			}

			fakeK8sClient := fake.NewSimpleClientset()

			err := ad.run(context.Background(), tt.dev, tt.namespace, "test-manifest", fakeK8sClient)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			at.AssertExpectations(t)
		})
	}
}

func TestAutoDownRunner_Run_WithAppNotFound(t *testing.T) {
	// Setup
	ioCtrl := io.NewIOController()
	k8sLogger := io.NewK8sLogger()
	at := &mockAnalyticsTracker{}
	downCmd := &mockDownCmdRunner{}
	downCmd.On("Run", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(assert.AnError)

	ad := &autoDownRunner{
		autoDown:         true,
		ioCtrl:           ioCtrl,
		k8sLogger:        k8sLogger,
		analyticsTracker: at,
		downCmd:          downCmd,
	}

	dev := &model.Dev{
		Name: "test-dev",
	}
	namespace := "test-namespace"

	fakeK8sClient := fake.NewSimpleClientset()

	err := ad.run(context.Background(), dev, namespace, "test-manifest", fakeK8sClient)

	// Should not error as not found is handled gracefully
	assert.ErrorIs(t, err, assert.AnError)
	at.AssertExpectations(t)
}
