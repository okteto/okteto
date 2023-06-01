package v2

import (
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

type fakeConfigRegistry struct {
	access bool
	err    error
}

func (fcr fakeConfigRegistry) HasGlobalPushAccess() (bool, error) { return fcr.access, fcr.err }

type fakeConfigRepo struct {
	sha     string
	isClean bool
	err     error
}

func (fcr fakeConfigRepo) GetSHA() (string, error) { return fcr.sha, fcr.err }
func (fcr fakeConfigRepo) IsClean() (bool, error)  { return fcr.isClean, fcr.err }

func TestGetConfig(t *testing.T) {
	type input struct {
		reg  fakeConfigRegistry
		repo fakeConfigRepo
	}
	tt := []struct {
		name     string
		input    input
		expected oktetoBuilderConfig
	}{
		{
			name: "global access clean commit",
			input: input{
				reg: fakeConfigRegistry{
					access: true,
					err:    nil,
				},
				repo: fakeConfigRepo{
					isClean: true,
					err:     nil,
				},
			},
			expected: oktetoBuilderConfig{
				hasGlobalAccess: true,
				isCleanProject:  true,
				repository: fakeConfigRepo{
					isClean: true,
					err:     nil,
				},
				fs:       afero.NewOsFs(),
				isOkteto: true,
			},
		},
		{
			name: "no global access clean commit",
			input: input{
				reg: fakeConfigRegistry{
					access: false,
					err:    nil,
				},
				repo: fakeConfigRepo{
					isClean: true,
					err:     nil,
				},
			},
			expected: oktetoBuilderConfig{
				hasGlobalAccess: false,
				isCleanProject:  true,
				repository: fakeConfigRepo{
					isClean: true,
					err:     nil,
				},
				fs:       afero.NewOsFs(),
				isOkteto: true,
			},
		},
		{
			name: "error on global access clean commit",
			input: input{
				reg: fakeConfigRegistry{
					access: false,
					err:    assert.AnError,
				},
				repo: fakeConfigRepo{
					isClean: true,
					err:     nil,
				},
			},
			expected: oktetoBuilderConfig{
				hasGlobalAccess: false,
				isCleanProject:  true,
				repository: fakeConfigRepo{
					isClean: true,
					err:     nil,
				},
				fs:       afero.NewOsFs(),
				isOkteto: true,
			},
		},
		{
			name: "error on clean commit and global access",
			input: input{
				reg: fakeConfigRegistry{
					access: false,
					err:    assert.AnError,
				},
				repo: fakeConfigRepo{
					isClean: false,
					err:     assert.AnError,
				},
			},
			expected: oktetoBuilderConfig{
				hasGlobalAccess: false,
				isCleanProject:  false,
				repository: fakeConfigRepo{
					isClean: false,
					err:     assert.AnError,
				},
				fs:       afero.NewOsFs(),
				isOkteto: true,
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cfg := getConfig(tc.input.reg, tc.input.repo)
			assert.Equal(t, tc.expected, cfg)
		})
	}
}

func TestGetGitCommit(t *testing.T) {
	tt := []struct {
		name     string
		input    fakeConfigRepo
		expected string
	}{
		{
			name: "valid commit",
			input: fakeConfigRepo{
				sha:     "1234567890",
				isClean: true,
				err:     nil,
			},
			expected: "1234567890",
		},
		{
			name: "invalid commit",
			input: fakeConfigRepo{
				sha:     "",
				isClean: true,
				err:     assert.AnError,
			},
			expected: "",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cfg := oktetoBuilderConfig{
				repository: tc.input,
			}
			assert.Equal(t, tc.expected, cfg.GetGitCommit())
		})
	}
}

func TestGetTextToHash(t *testing.T) {
	fs := afero.NewMemMapFs()
	afero.WriteFile(fs, "secret", []byte("bar"), 0600)
	t.Setenv("BAR", "bar")
	type input struct {
		repo      fakeConfigRepo
		buildInfo *model.BuildInfo
	}
	tt := []struct {
		name     string
		input    input
		expected string
	}{
		{
			name: "valid commit",
			input: input{
				repo: fakeConfigRepo{
					sha:     "1234567890",
					isClean: true,
					err:     nil,
				},
				buildInfo: &model.BuildInfo{
					Args: model.BuildArgs{
						{
							Name:  "foo",
							Value: "bar",
						},
						{
							Name:  "key",
							Value: "value",
						},
					},
					Target: "target",
					Secrets: model.BuildSecrets{
						"secret": "secret",
					},
					Context:    "context",
					Dockerfile: "dockerfile",
					Image:      "image",
				},
			},
			expected: "commit:1234567890;target:target;build_args:foo=bar;key=value;secrets:secret=secret;context:context;dockerfile:dockerfile;image:image;",
		},
		{
			name: "invalid commit",
			input: input{
				repo: fakeConfigRepo{
					sha:     "",
					isClean: true,
					err:     assert.AnError,
				},
				buildInfo: &model.BuildInfo{
					Args: model.BuildArgs{
						{
							Name:  "foo",
							Value: "bar",
						},
						{
							Name:  "key",
							Value: "value",
						},
					},
					Target: "target",
					Secrets: model.BuildSecrets{
						"secret": "secret",
					},
					Context:    "context",
					Dockerfile: "dockerfile",
					Image:      "image",
				},
			},
			expected: "commit:;target:target;build_args:foo=bar;key=value;secrets:secret=secret;context:context;dockerfile:dockerfile;image:image;",
		},
		{
			name: "invalid commit and no args",
			input: input{
				repo: fakeConfigRepo{
					sha:     "",
					isClean: true,
					err:     assert.AnError,
				},
				buildInfo: &model.BuildInfo{
					Args:   model.BuildArgs{},
					Target: "target",
					Secrets: model.BuildSecrets{
						"secret": "secret",
					},
					Context:    "context",
					Dockerfile: "dockerfile",
					Image:      "image",
				},
			},
			expected: "commit:;target:target;build_args:;secrets:secret=secret;context:context;dockerfile:dockerfile;image:image;",
		},
		{
			name: "arg with expansion",
			input: input{
				repo: fakeConfigRepo{
					sha:     "",
					isClean: true,
					err:     assert.AnError,
				},
				buildInfo: &model.BuildInfo{
					Args: model.BuildArgs{
						{
							Name:  "foo",
							Value: "$BAR",
						},
					},
					Target: "target",
					Secrets: model.BuildSecrets{
						"secret": "secret",
					},
					Context:    "context",
					Dockerfile: "dockerfile",
					Image:      "image",
				},
			},
			expected: "commit:;target:target;build_args:foo=bar;secrets:secret=secret;context:context;dockerfile:dockerfile;image:image;",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cfg := oktetoBuilderConfig{
				repository: tc.input.repo,
				fs:         fs,
			}
			assert.Equal(t, tc.expected, cfg.getTextToHash(tc.input.buildInfo, tc.input.repo.sha))
		})
	}
}

func TestGetBuildHash(t *testing.T) {
	tt := []struct {
		name        string
		input       fakeConfigRepo
		expectedLen int
	}{
		{
			name: "valid commit",
			input: fakeConfigRepo{
				sha:     "1234567890",
				isClean: true,
				err:     nil,
			},
			expectedLen: 64,
		},
		{
			name: "invalid commit",
			input: fakeConfigRepo{
				sha:     "",
				isClean: true,
				err:     assert.AnError,
			},
			expectedLen: 0,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cfg := oktetoBuilderConfig{
				repository: tc.input,
				fs:         afero.Afero{},
			}
			firstExecution := cfg.GetBuildHash(&model.BuildInfo{})
			assert.Len(t, firstExecution, tc.expectedLen)
			assert.Equal(t, firstExecution, cfg.GetBuildHash(&model.BuildInfo{}))
		})
	}
}
