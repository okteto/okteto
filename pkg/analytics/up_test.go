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

package analytics

import (
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/deps"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_UpMetricsMetadata_ManifestProps(t *testing.T) {
	tests := []struct {
		manifest *model.Manifest
		expected *UpMetricsMetadata
		name     string
	}{
		{
			name: "manifest with build section",
			manifest: &model.Manifest{
				Build: build.ManifestBuild{
					"service": &build.Info{
						Context: "service",
					},
				},
			},
			expected: &UpMetricsMetadata{
				hasBuildSection: true,
			},
		},
		{
			name: "manifest with dependencies section",
			manifest: &model.Manifest{
				Dependencies: deps.ManifestSection{
					"service": &deps.Dependency{},
				},
			},
			expected: &UpMetricsMetadata{
				hasDependenciesSection: true,
			},
		},
		{
			name: "manifest with deploy section",
			manifest: &model.Manifest{
				Deploy: &model.DeployInfo{
					Commands: []model.DeployCommand{
						{
							Name:    "my command",
							Command: "echo test",
						},
					},
				},
			},
			expected: &UpMetricsMetadata{
				hasDeploySection: true,
			},
		},
		{
			name: "manifest type",
			manifest: &model.Manifest{
				Type: model.OktetoManifestType,
			},
			expected: &UpMetricsMetadata{
				manifestType: "manifest",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &UpMetricsMetadata{}
			m.ManifestProps(tt.manifest)
			assert.Equal(t, tt.expected, m)
		})
	}

}

func Test_UpMetricsMetadata_DevProps(t *testing.T) {
	tests := []struct {
		dev      *model.Dev
		expected *UpMetricsMetadata
		name     string
	}{
		{
			name: "dev interactive sync mode",
			dev: &model.Dev{
				Mode: "sync",
			},
			expected: &UpMetricsMetadata{
				mode:          "sync",
				isInteractive: true,
			},
		},
		{
			name: "dev interactive hybrid mode",
			dev: &model.Dev{
				Mode: "hybrid",
			},
			expected: &UpMetricsMetadata{
				mode:          "hybrid",
				isInteractive: true,
			},
		},
		{
			name: "dev interactive",
			dev:  &model.Dev{},
			expected: &UpMetricsMetadata{
				isInteractive: true,
			},
		},
		{
			name: "dev not interactive",
			dev: &model.Dev{
				Command: model.Command{
					Values: []string{"yarn start"},
				},
			},
			expected: &UpMetricsMetadata{},
		},
		{
			name: "dev interactive with reverse",
			dev: &model.Dev{
				Reverse: []model.Reverse{
					{
						Remote: 8080,
						Local:  8080,
					},
				},
			},
			expected: &UpMetricsMetadata{
				hasReverse:    true,
				isInteractive: true,
			},
		},
		{
			name: "dev with service name",
			dev: &model.Dev{
				Name: "api",
				Mode: "sync",
			},
			expected: &UpMetricsMetadata{
				service:       "api",
				mode:          "sync",
				isInteractive: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &UpMetricsMetadata{}
			m.DevProps(tt.dev)
			assert.Equal(t, tt.expected, m)
		})
	}

}

func Test_UpMetricsMetadata_RepositoryProps(t *testing.T) {
	tests := []struct {
		expected           *UpMetricsMetadata
		name               string
		isOktetoRepository bool
	}{
		{
			name:               "is okteto repository",
			isOktetoRepository: true,
			expected: &UpMetricsMetadata{
				isOktetoRepository: true,
			},
		},
		{
			name:               "is not okteto repository",
			isOktetoRepository: false,
			expected: &UpMetricsMetadata{
				isOktetoRepository: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &UpMetricsMetadata{}
			m.RepositoryProps(tt.isOktetoRepository)
			assert.Equal(t, tt.expected, m)
		})
	}
}

func Test_UpMetricsMetadata_ReconnectDefault(t *testing.T) {
	m := &UpMetricsMetadata{}
	m.ReconnectDefault()
	assert.Equal(t, &UpMetricsMetadata{
		isReconnect:    true,
		reconnectCause: "unrecognised",
		reconnectCount: 1,
	}, m)
}

func Test_UpMetricsMetadata_ReconnectDefault_MultipleReconnects(t *testing.T) {
	m := &UpMetricsMetadata{}
	m.ReconnectDefault()
	m.ReconnectDevPodRecreated()
	assert.Equal(t, &UpMetricsMetadata{
		isReconnect:    true,
		reconnectCause: "dev-pod-recreated",
		reconnectCount: 2,
	}, m)
}

func Test_UpMetricsMetadata_ReconnectDevPodRecreated(t *testing.T) {
	m := &UpMetricsMetadata{}
	m.ReconnectDevPodRecreated()
	assert.Equal(t, &UpMetricsMetadata{
		isReconnect:    true,
		reconnectCause: "dev-pod-recreated",
		reconnectCount: 1,
	}, m)
}

func Test_UpMetricsMetadata_Errors(t *testing.T) {
	m := &UpMetricsMetadata{}
	m.ErrSync()
	m.ErrSyncResetDatabase()
	m.FailActivate()
	assert.Equal(t, &UpMetricsMetadata{
		errSync:              true,
		errSyncResetDatabase: true,
		failActivate:         true,
	}, m)
}

func Test_UpMetricsMetadata_CommandSuccess(t *testing.T) {
	m := &UpMetricsMetadata{}
	m.CommandSuccess()
	assert.Equal(t, &UpMetricsMetadata{
		success: true,
	}, m)
}

func Test_UpMetricsMetadata_ErrorReason(t *testing.T) {
	tests := []struct {
		name     string
		meta     UpMetricsMetadata
		expected string
	}{
		{name: "no error", meta: UpMetricsMetadata{}, expected: ""},
		{
			name:     "fail_activate takes precedence over errSync",
			meta:     UpMetricsMetadata{failActivate: true, errSync: true},
			expected: "fail_activate",
		},
		{
			name:     "insufficient space",
			meta:     UpMetricsMetadata{errSyncInsufficientSpace: true},
			expected: "err_sync_insufficient_space",
		},
		{
			name:     "reset database",
			meta:     UpMetricsMetadata{errSyncResetDatabase: true},
			expected: "err_sync_reset_database",
		},
		{
			name:     "lost syncthing",
			meta:     UpMetricsMetadata{errSyncLostSyncthing: true},
			expected: "err_sync_lost_syncthing",
		},
		{
			name:     "generic sync error",
			meta:     UpMetricsMetadata{errSync: true},
			expected: "err_sync",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.meta.errorReason())
		})
	}
}

func Test_UpMetricsMetadata_ToPostHogProps(t *testing.T) {
	baseProps := func(overrides map[string]any) map[string]any {
		base := map[string]any{
			"result":                        false,
			"manifest_type":                 "",
			"is_interactive":                false,
			"is_build_executed":             false,
			"is_deploy_executed":            false,
			"has_build_section":             false,
			"has_deploy_section":            false,
			"is_reconnect":                  false,
			"reconnect_count":               0,
			"is_auto_down":                  false,
			"workflow_id":                   "",
			"service":                       "",
			"repo_url":                      "",
			"duration_seconds":              0,
			"initial_sync_duration_seconds": 0,
			"dev_container_creation_duration_seconds": 0,
		}
		for k, v := range overrides {
			base[k] = v
		}
		return base
	}

	tests := []struct {
		name     string
		meta     UpMetricsMetadata
		expected map[string]any
	}{
		{
			name:     "minimal — all fields present with zero values",
			meta:     UpMetricsMetadata{success: true},
			expected: baseProps(map[string]any{"result": true}),
		},
		{
			name: "service and repo_url set; namespace excluded (resolved via withNamespace enricher)",
			meta: UpMetricsMetadata{success: true, service: "api", namespace: "dev-ns", repoURL: "https://github.com/org/repo"},
			expected: baseProps(map[string]any{
				"result":   true,
				"service":  "api",
				"repo_url": "bdb72e6e68b80f9ed3bbdb0ad1d2f8b4fac8ade379eb82182de40a3357a2d3b3",
			}),
		},
		{
			name:     "failure sets error_reason",
			meta:     UpMetricsMetadata{success: false, failActivate: true},
			expected: baseProps(map[string]any{"error_reason": "fail_activate"}),
		},
		{
			name:     "error_reason empty on success",
			meta:     UpMetricsMetadata{success: true, failActivate: true},
			expected: baseProps(map[string]any{"result": true}),
		},
		{
			name: "reconnect_cause populated when is_reconnect",
			meta: UpMetricsMetadata{
				success:        true,
				isReconnect:    true,
				reconnectCount: 1,
				reconnectCause: reconnectCauseDevPodRecreated,
			},
			expected: baseProps(map[string]any{
				"result":          true,
				"is_reconnect":    true,
				"reconnect_count": 1,
				"reconnect_cause": "dev-pod-recreated",
			}),
		},
		{
			name: "durations always included",
			meta: UpMetricsMetadata{
				success:                      true,
				execDuration:                 60 * time.Second,
				initialSyncDuration:          10 * time.Second,
				devContainerCreationDuration: 5 * time.Second,
			},
			expected: baseProps(map[string]any{
				"result":                                  true,
				"duration_seconds":                        60,
				"initial_sync_duration_seconds":           10,
				"dev_container_creation_duration_seconds": 5,
			}),
		},
		{
			name: "is_build_executed and has_build/deploy_section flags",
			meta: UpMetricsMetadata{
				success:          true,
				isBuildExecuted:  true,
				hasBuildSection:  true,
				hasDeploySection: true,
			},
			expected: baseProps(map[string]any{
				"result":             true,
				"is_build_executed":  true,
				"has_build_section":  true,
				"has_deploy_section": true,
			}),
		},
		{
			name: "all flags",
			meta: UpMetricsMetadata{
				success:           true,
				manifestType:      model.OktetoManifestType,
				isInteractive:     true,
				isBuildExecuted:   true,
				hasRunDeploy:      true,
				hasBuildSection:   true,
				hasDeploySection:  true,
				isReconnect:       true,
				reconnectCount:    2,
				reconnectCause:    reconnectCauseDefault,
				isAutoDownEnabled: true,
				service:           "api",
				namespace:         "my-ns",
			},
			expected: baseProps(map[string]any{
				"result":             true,
				"manifest_type":      "manifest",
				"is_interactive":     true,
				"is_build_executed":  true,
				"is_deploy_executed": true,
				"has_build_section":  true,
				"has_deploy_section": true,
				"is_reconnect":       true,
				"reconnect_count":    2,
				"is_auto_down":       true,
				"reconnect_cause":    "unrecognised",
				"service":            "api",
			}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.meta.toPostHogProps())
		})
	}
}

func Test_UpTracker(t *testing.T) {
	tests := []struct {
		expected mockEvent
		name     string
		meta     UpMetricsMetadata
	}{
		{
			name: "empty event",
			meta: UpMetricsMetadata{},
			expected: mockEvent{
				event:   "Up",
				success: false,
				props: map[string]any{
					"activateDurationSeconds":             float64(0),
					"errSyncResetDatabase":                false,
					"errSync":                             false,
					"failActivate":                        false,
					"hasBuildSection":                     false,
					"hasDependenciesSection":              false,
					"hasDeploySection":                    false,
					"hasReverse":                          false,
					"initialSyncDurationSeconds":          float64(0),
					"isInteractive":                       false,
					"isOktetoRepository":                  false,
					"isReconnect":                         false,
					"manifestType":                        model.Archetype(""),
					"mode":                                "",
					"reconnectCause":                      "",
					"contextSyncDurationSeconds":          float64(0),
					"devContainerCreationDurationSeconds": float64(0),
					"execDurationSeconds":                 float64(0),
					"hasRunDeploy":                        false,
					"localFoldersScanDurationSeconds":     float64(0),
					"oktetoCtxConfigDurationSeconds":      float64(0),
					"errSyncInsufficientSpace":            false,
					"errSyncLostSyncthing":                false,
					"isAutoDownEnabled":                   false,
				},
			},
		},
		{
			name: "command success empty event",
			meta: UpMetricsMetadata{
				success: true,
			},
			expected: mockEvent{
				event:   "Up",
				success: true,
				props: map[string]any{
					"activateDurationSeconds":             float64(0),
					"errSyncResetDatabase":                false,
					"errSync":                             false,
					"failActivate":                        false,
					"hasBuildSection":                     false,
					"hasDependenciesSection":              false,
					"hasDeploySection":                    false,
					"hasReverse":                          false,
					"initialSyncDurationSeconds":          float64(0),
					"isInteractive":                       false,
					"isOktetoRepository":                  false,
					"isReconnect":                         false,
					"manifestType":                        model.Archetype(""),
					"mode":                                "",
					"reconnectCause":                      "",
					"contextSyncDurationSeconds":          float64(0),
					"devContainerCreationDurationSeconds": float64(0),
					"execDurationSeconds":                 float64(0),
					"hasRunDeploy":                        false,
					"localFoldersScanDurationSeconds":     float64(0),
					"oktetoCtxConfigDurationSeconds":      float64(0),
					"errSyncInsufficientSpace":            false,
					"errSyncLostSyncthing":                false,
					"isAutoDownEnabled":                   false,
				},
			},
		},
		{
			name: "command success all fields",
			meta: UpMetricsMetadata{
				manifestType:                 model.OktetoManifestType,
				isInteractive:                true,
				isOktetoRepository:           true,
				hasDependenciesSection:       true,
				hasBuildSection:              true,
				hasDeploySection:             true,
				hasReverse:                   true,
				isHybridDev:                  true,
				mode:                         "sync",
				activateDuration:             1 * time.Minute,
				initialSyncDuration:          1 * time.Minute,
				success:                      true,
				hasRunDeploy:                 true,
				contextSyncDuration:          1 * time.Minute,
				devContainerCreationDuration: 1 * time.Minute,
				execDuration:                 1 * time.Minute,
				localFoldersScanDuration:     1 * time.Minute,
				oktetoCtxConfigDuration:      1 * time.Minute,
			},
			expected: mockEvent{
				event:   "Up",
				success: true,
				props: map[string]any{
					"activateDurationSeconds":             float64(60),
					"errSyncResetDatabase":                false,
					"errSync":                             false,
					"failActivate":                        false,
					"hasBuildSection":                     true,
					"hasDependenciesSection":              true,
					"hasDeploySection":                    true,
					"hasReverse":                          true,
					"initialSyncDurationSeconds":          float64(60),
					"isInteractive":                       true,
					"isOktetoRepository":                  true,
					"isReconnect":                         false,
					"manifestType":                        model.Archetype("manifest"),
					"mode":                                "sync",
					"reconnectCause":                      "",
					"contextSyncDurationSeconds":          float64(60),
					"devContainerCreationDurationSeconds": float64(60),
					"execDurationSeconds":                 float64(60),
					"hasRunDeploy":                        true,
					"localFoldersScanDurationSeconds":     float64(60),
					"oktetoCtxConfigDurationSeconds":      float64(60),
					"errSyncInsufficientSpace":            false,
					"errSyncLostSyncthing":                false,
					"isAutoDownEnabled":                   false,
				},
			},
		},
		{
			name: "command not success with errors",
			meta: UpMetricsMetadata{
				manifestType:           model.OktetoManifestType,
				isInteractive:          true,
				isOktetoRepository:     true,
				hasDependenciesSection: true,
				hasBuildSection:        true,
				hasDeploySection:       true,
				hasReverse:             true,
				isHybridDev:            true,
				mode:                   "sync",
				activateDuration:       1 * time.Minute,
				initialSyncDuration:    1 * time.Minute,
				success:                false,
				errSync:                true,
				errSyncResetDatabase:   true,
				failActivate:           true,
			},
			expected: mockEvent{
				event:   "Up",
				success: false,
				props: map[string]any{
					"activateDurationSeconds":             float64(60),
					"errSyncResetDatabase":                true,
					"errSync":                             true,
					"failActivate":                        true,
					"hasBuildSection":                     true,
					"hasDependenciesSection":              true,
					"hasDeploySection":                    true,
					"hasReverse":                          true,
					"initialSyncDurationSeconds":          float64(60),
					"isInteractive":                       true,
					"isOktetoRepository":                  true,
					"isReconnect":                         false,
					"manifestType":                        model.Archetype("manifest"),
					"mode":                                "sync",
					"reconnectCause":                      "",
					"contextSyncDurationSeconds":          float64(0),
					"devContainerCreationDurationSeconds": float64(0),
					"execDurationSeconds":                 float64(0),
					"hasRunDeploy":                        false,
					"localFoldersScanDurationSeconds":     float64(0),
					"oktetoCtxConfigDurationSeconds":      float64(0),
					"errSyncInsufficientSpace":            false,
					"errSyncLostSyncthing":                false,
					"isAutoDownEnabled":                   false,
				},
			},
		},
		{
			name: "command success all fields with reconnect",
			meta: UpMetricsMetadata{
				manifestType:           model.OktetoManifestType,
				isInteractive:          true,
				isOktetoRepository:     true,
				hasDependenciesSection: true,
				hasBuildSection:        true,
				hasDeploySection:       true,
				hasReverse:             true,
				isHybridDev:            true,
				mode:                   "sync",
				activateDuration:       1 * time.Minute,
				initialSyncDuration:    1 * time.Minute,
				success:                true,
				isReconnect:            true,
				reconnectCause:         reconnectCauseDefault,
			},
			expected: mockEvent{
				event:   "Up",
				success: true,
				props: map[string]any{
					"activateDurationSeconds":             float64(60),
					"errSyncResetDatabase":                false,
					"errSync":                             false,
					"failActivate":                        false,
					"hasBuildSection":                     true,
					"hasDependenciesSection":              true,
					"hasDeploySection":                    true,
					"hasReverse":                          true,
					"initialSyncDurationSeconds":          float64(60),
					"isInteractive":                       true,
					"isOktetoRepository":                  true,
					"isReconnect":                         true,
					"manifestType":                        model.Archetype("manifest"),
					"mode":                                "sync",
					"reconnectCause":                      "unrecognised",
					"contextSyncDurationSeconds":          float64(0),
					"devContainerCreationDurationSeconds": float64(0),
					"execDurationSeconds":                 float64(0),
					"hasRunDeploy":                        false,
					"localFoldersScanDurationSeconds":     float64(0),
					"oktetoCtxConfigDurationSeconds":      float64(0),
					"errSyncInsufficientSpace":            false,
					"errSyncLostSyncthing":                false,
					"isAutoDownEnabled":                   false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventMeta := &mockEvent{}
			tracker := Tracker{
				trackFn: func(event string, success bool, props map[string]any) {
					eventMeta = &mockEvent{
						event:   event,
						success: success,
						props:   props,
					}
				},
			}

			tracker.TrackUp(&tt.meta)
			assert.Equal(t, tt.expected.event, eventMeta.event)
			assert.Equal(t, tt.expected.success, eventMeta.success)
			assert.Equal(t, tt.expected.props, eventMeta.props)
		})

	}
}

func TestAnalyticsTracker_TrackUpStarted(t *testing.T) {
	tests := []struct {
		name       string
		service    string
		namespace  string
		repoURL    string
		workflowID string
	}{
		{
			name:       "all fields dispatched to backend",
			service:    "api",
			namespace:  "dev-ns",
			repoURL:    "https://github.com/org/repo",
			workflowID: "abc-123",
		},
		{
			name:       "empty fields dispatched to backend",
			service:    "",
			namespace:  "",
			repoURL:    "",
			workflowID: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedService, capturedNamespace, capturedRepoURL, capturedWorkflowID string
			mock := &mockAnalyticsBackend{
				trackUpStartedFn: func(service, namespace, repoURL, workflowID string) {
					capturedService = service
					capturedNamespace = namespace
					capturedRepoURL = repoURL
					capturedWorkflowID = workflowID
				},
			}
			tracker := &Tracker{
				trackFn:  func(_ string, _ bool, _ map[string]any) {},
				backends: []analyticsBackend{mock},
			}
			tracker.TrackUpStarted(tt.service, tt.namespace, tt.repoURL, tt.workflowID)

			require.Equal(t, tt.service, capturedService)
			require.Equal(t, tt.namespace, capturedNamespace)
			require.Equal(t, tt.repoURL, capturedRepoURL)
			require.Equal(t, tt.workflowID, capturedWorkflowID)
		})
	}
}

func TestAnalyticsTracker_TrackUp(t *testing.T) {
	tests := []struct {
		input           *UpMetricsMetadata
		expectedSuccess bool
		name            string
	}{
		{
			name:            "success event dispatched to backend",
			input:           &UpMetricsMetadata{success: true},
			expectedSuccess: true,
		},
		{
			name:            "failure event dispatched to backend",
			input:           &UpMetricsMetadata{success: false},
			expectedSuccess: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedMeta *UpMetricsMetadata
			mock := &mockAnalyticsBackend{
				trackUpFn: func(m *UpMetricsMetadata) {
					capturedMeta = m
				},
			}
			tracker := &Tracker{
				trackFn:  func(_ string, _ bool, _ map[string]any) {},
				backends: []analyticsBackend{mock},
			}
			tracker.TrackUp(tt.input)

			require.NotNil(t, capturedMeta)
			require.Equal(t, tt.expectedSuccess, capturedMeta.success)
		})
	}
}
