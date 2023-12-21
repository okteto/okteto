package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildInfo_GetDockerfilePath(t *testing.T) {
	dir := t.TempDir()

	dockerfilePath := filepath.Join(dir, "Dockerfile")
	dockerfiledevPath := filepath.Join(dir, "Dockerfile.dev")
	assert.NoError(t, os.WriteFile(dockerfilePath, []byte(`FROM alpine`), 0600))
	assert.NoError(t, os.WriteFile(dockerfiledevPath, []byte(`FROM alpine`), 0600))
	tests := []struct {
		name       string
		context    string
		dockerfile string
		want       string
	}{
		{
			name:       "with-context",
			context:    dir,
			dockerfile: "Dockerfile",
			want:       filepath.Join(dir, "Dockerfile"),
		},
		{
			name:       "with-context-and-non-dockerfile",
			context:    dir,
			dockerfile: "Dockerfile.dev",
			want:       filepath.Join(dir, "Dockerfile.dev"),
		},
		{
			name:       "empty",
			context:    "",
			dockerfile: "",
			want:       "",
		},
		{
			name:       "default",
			context:    "",
			dockerfile: "Dockerfile",
			want:       "Dockerfile",
		},

		{
			name:       "with-context-and-dockerfile-expanded",
			context:    "api",
			dockerfile: "api/Dockerfile.dev",
			want:       "api/Dockerfile.dev",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &BuildInfo{
				Context:    tt.context,
				Dockerfile: tt.dockerfile,
			}
			if got := b.GetDockerfilePath(); got != tt.want {
				t.Errorf("BuildInfo.GetDockerfilePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_BuildInfoCopy(t *testing.T) {
	b := &BuildInfo{
		Name:        "test",
		Context:     "context",
		Dockerfile:  "dockerfile",
		Target:      "target",
		Image:       "image",
		CacheFrom:   []string{"cache"},
		ExportCache: []string{"export"},
		Args: BuildArgs{
			BuildArg{
				Name:  "env",
				Value: "test",
			},
		},
		Secrets: BuildSecrets{
			"sec": "test",
		},
		VolumesToInclude: []VolumeMounts{
			{
				LocalPath:  "local",
				RemotePath: "remote",
			},
		},
		DependsOn: BuildDependsOn{"other"},
	}

	copyB := b.Copy()
	assert.EqualValues(t, b, copyB)

	samePointer := &copyB == &b
	assert.False(t, samePointer)
}

func TestExpandBuildArgs(t *testing.T) {
	t.Setenv("KEY", "VALUE")
	tests := []struct {
		buildInfo          *BuildInfo
		expected           *BuildInfo
		previousImageBuilt map[string]string
		name               string
	}{
		{
			name:               "no build args",
			buildInfo:          &BuildInfo{},
			previousImageBuilt: map[string]string{},
			expected:           &BuildInfo{},
		},
		{
			name: "only buildInfo without expanding",
			buildInfo: &BuildInfo{
				Args: BuildArgs{
					{
						Name:  "KEY",
						Value: "VALUE",
					},
				},
			},
			previousImageBuilt: map[string]string{},
			expected: &BuildInfo{
				Args: BuildArgs{
					{
						Name:  "KEY",
						Value: "VALUE",
					},
				},
			},
		},
		{
			name: "only buildInfo expanding",
			buildInfo: &BuildInfo{
				Args: BuildArgs{
					{
						Name:  "KEY",
						Value: "$KEY",
					},
				},
			},
			previousImageBuilt: map[string]string{},
			expected: &BuildInfo{
				Args: BuildArgs{
					{
						Name:  "KEY",
						Value: "VALUE",
					},
				},
			},
		},
		{
			name:      "only previousImageBuilt",
			buildInfo: &BuildInfo{},
			previousImageBuilt: map[string]string{
				"KEY": "VALUE",
			},
			expected: &BuildInfo{
				Args: BuildArgs{
					{
						Name:  "KEY",
						Value: "VALUE",
					},
				},
			},
		},
		{
			name: "buildInfo args and previousImageBuilt without expanding",
			buildInfo: &BuildInfo{
				Args: BuildArgs{
					{
						Name:  "KEY",
						Value: "VALUE",
					},
				},
			},
			previousImageBuilt: map[string]string{
				"KEY2": "VALUE2",
			},
			expected: &BuildInfo{
				Args: BuildArgs{
					{
						Name:  "KEY",
						Value: "VALUE",
					},
					{
						Name:  "KEY2",
						Value: "VALUE2",
					},
				},
			},
		},
		{
			name: "buildInfo args and previousImageBuilt expanding",
			buildInfo: &BuildInfo{
				Args: BuildArgs{
					{
						Name:  "KEY",
						Value: "$KEY",
					},
				},
			},
			previousImageBuilt: map[string]string{
				"KEY2": "VALUE2",
			},
			expected: &BuildInfo{
				Args: BuildArgs{
					{
						Name:  "KEY",
						Value: "VALUE",
					},
					{
						Name:  "KEY2",
						Value: "VALUE2",
					},
				},
			},
		},
		{
			name: "buildInfo args only same as previousImageBuilt",
			buildInfo: &BuildInfo{
				Args: BuildArgs{
					{
						Name:  "KEY",
						Value: "$KEY",
					},
				},
			},
			previousImageBuilt: map[string]string{
				"KEY": "VALUE",
			},
			expected: &BuildInfo{
				Args: BuildArgs{
					{
						Name:  "KEY",
						Value: "VALUE",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, tt.buildInfo.AddBuildArgs(tt.previousImageBuilt))

			assert.Equal(t, tt.expected, tt.buildInfo)
		})
	}
}
