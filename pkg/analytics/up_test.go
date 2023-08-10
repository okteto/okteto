package analytics

import (
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
)

func Test_UpMetricsMetadata_ManifestProps(t *testing.T) {
	tests := []struct {
		name     string
		manifest *model.Manifest
		expected *UpMetricsMetadata
	}{
		{
			name: "manifest with build section",
			manifest: &model.Manifest{
				IsV2: true,
				Build: model.ManifestBuild{
					"service": &model.BuildInfo{
						Context: "service",
					},
				},
			},
			expected: &UpMetricsMetadata{
				isV2:            true,
				hasBuildSection: true,
			},
		},
		{
			name: "manifest with dependencies section",
			manifest: &model.Manifest{
				IsV2: true,
				Dependencies: model.ManifestDependencies{
					"service": &model.Dependency{},
				},
			},
			expected: &UpMetricsMetadata{
				isV2:                   true,
				hasDependenciesSection: true,
			},
		},
		{
			name: "manifest with deploy section",
			manifest: &model.Manifest{
				IsV2: true,
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
				isV2:             true,
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
		name     string
		dev      *model.Dev
		expected *UpMetricsMetadata
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
		name               string
		isOktetoRepository bool
		expected           *UpMetricsMetadata
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
	}, m)
}

func Test_UpMetricsMetadata_ReconnectDevPodRecreated(t *testing.T) {
	m := &UpMetricsMetadata{}
	m.ReconnectDevPodRecreated()
	assert.Equal(t, &UpMetricsMetadata{
		isReconnect:    true,
		reconnectCause: "dev-pod-recreated",
	}, m)
}

func Test_UpMetricsMetadata_Errors(t *testing.T) {
	m := &UpMetricsMetadata{}
	m.ErrSync()
	m.ErrResetDatabase()
	m.FailActivate()
	assert.Equal(t, &UpMetricsMetadata{
		errSync:          true,
		errResetDatabase: true,
		failActivate:     true,
	}, m)
}

func Test_UpMetricsMetadata_CommandSuccess(t *testing.T) {
	m := &UpMetricsMetadata{}
	m.CommandSuccess()
	assert.Equal(t, &UpMetricsMetadata{
		success: true,
	}, m)
}

func Test_UpTracker(t *testing.T) {
	tests := []struct {
		name     string
		meta     UpMetricsMetadata
		expected mockEvent
	}{
		{
			name: "empty event",
			meta: UpMetricsMetadata{},
			expected: mockEvent{
				event:   "Up",
				success: false,
				props: map[string]interface{}{
					"activateDurationSeconds":    float64(0),
					"errResetDatabase":           false,
					"errSync":                    false,
					"failActivate":               false,
					"hasBuildSection":            false,
					"hasDependenciesSection":     false,
					"hasDeploySection":           false,
					"hasReverse":                 false,
					"initialSyncDurationSeconds": float64(0),
					"isInteractive":              false,
					"isOktetoRepository":         false,
					"isReconnect":                false,
					"isV2":                       false,
					"manifestType":               model.Archetype(""),
					"mode":                       "",
					"reconnectCause":             "",
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
				props: map[string]interface{}{
					"activateDurationSeconds":    float64(0),
					"errResetDatabase":           false,
					"errSync":                    false,
					"failActivate":               false,
					"hasBuildSection":            false,
					"hasDependenciesSection":     false,
					"hasDeploySection":           false,
					"hasReverse":                 false,
					"initialSyncDurationSeconds": float64(0),
					"isInteractive":              false,
					"isOktetoRepository":         false,
					"isReconnect":                false,
					"isV2":                       false,
					"manifestType":               model.Archetype(""),
					"mode":                       "",
					"reconnectCause":             "",
				},
			},
		},
		{
			name: "command success all fields",
			meta: UpMetricsMetadata{
				isV2:                   true,
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
			},
			expected: mockEvent{
				event:   "Up",
				success: true,
				props: map[string]interface{}{
					"activateDurationSeconds":    float64(60),
					"errResetDatabase":           false,
					"errSync":                    false,
					"failActivate":               false,
					"hasBuildSection":            true,
					"hasDependenciesSection":     true,
					"hasDeploySection":           true,
					"hasReverse":                 true,
					"initialSyncDurationSeconds": float64(60),
					"isInteractive":              true,
					"isOktetoRepository":         true,
					"isReconnect":                false,
					"isV2":                       true,
					"manifestType":               model.Archetype("manifest"),
					"mode":                       "sync",
					"reconnectCause":             "",
				},
			},
		},
		{
			name: "command not success with errors",
			meta: UpMetricsMetadata{
				isV2:                   true,
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
				errResetDatabase:       true,
				failActivate:           true,
			},
			expected: mockEvent{
				event:   "Up",
				success: false,
				props: map[string]interface{}{
					"activateDurationSeconds":    float64(60),
					"errResetDatabase":           true,
					"errSync":                    true,
					"failActivate":               true,
					"hasBuildSection":            true,
					"hasDependenciesSection":     true,
					"hasDeploySection":           true,
					"hasReverse":                 true,
					"initialSyncDurationSeconds": float64(60),
					"isInteractive":              true,
					"isOktetoRepository":         true,
					"isReconnect":                false,
					"isV2":                       true,
					"manifestType":               model.Archetype("manifest"),
					"mode":                       "sync",
					"reconnectCause":             "",
				},
			},
		},
		{
			name: "command success all fields with reconnect",
			meta: UpMetricsMetadata{
				isV2:                   true,
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
				props: map[string]interface{}{
					"activateDurationSeconds":    float64(60),
					"errResetDatabase":           false,
					"errSync":                    false,
					"failActivate":               false,
					"hasBuildSection":            true,
					"hasDependenciesSection":     true,
					"hasDeploySection":           true,
					"hasReverse":                 true,
					"initialSyncDurationSeconds": float64(60),
					"isInteractive":              true,
					"isOktetoRepository":         true,
					"isReconnect":                true,
					"isV2":                       true,
					"manifestType":               model.Archetype("manifest"),
					"mode":                       "sync",
					"reconnectCause":             "unrecognised",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventMeta := &mockEvent{}
			tracker := AnalyticsTracker{
				trackFn: func(event string, success bool, props map[string]interface{}) {
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
