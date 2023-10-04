package v2

import (
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestGetTextToHash(t *testing.T) {
	fs := afero.NewMemMapFs()
	err := afero.WriteFile(fs, "secret", []byte("bar"), 0600)
	assert.NoError(t, err)
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
			expected: "build_context:1234567890;target:target;build_args:foo=bar;key=value;secrets:secret=secret;context:context;dockerfile:dockerfile;image:image;",
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
			expected: "build_context:;target:target;build_args:foo=bar;key=value;secrets:secret=secret;context:context;dockerfile:dockerfile;image:image;",
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
			expected: "build_context:;target:target;build_args:;secrets:secret=secret;context:context;dockerfile:dockerfile;image:image;",
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
			expected: "build_context:;target:target;build_args:foo=bar;secrets:secret=secret;context:context;dockerfile:dockerfile;image:image;",
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, getTextToHash(tc.input.buildInfo, tc.input.repo.sha))
		})
	}
}
