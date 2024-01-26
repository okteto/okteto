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

package deps

import (
	"testing"
	"time"

	giturls "github.com/chainguard-dev/git-urls"
	"github.com/okteto/okteto/pkg/env"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func Test_GetTimeout(t *testing.T) {
	tests := []struct {
		dependency     *Dependency
		name           string
		defaultTimeout time.Duration
		expected       time.Duration
	}{
		{
			name:           "default timeout set and specific not",
			defaultTimeout: 5 * time.Minute,
			dependency:     &Dependency{},
			expected:       5 * time.Minute,
		},
		{
			name: "default timeout unset and specific set",
			dependency: &Dependency{
				Timeout: 10 * time.Minute,
			},
			expected: 10 * time.Minute,
		},
		{
			name:       "both unset",
			dependency: &Dependency{},
			expected:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.dependency.GetTimeout(tt.defaultTimeout)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_ExpandVars(t *testing.T) {
	t.Setenv("MY_CUSTOM_VAR_FROM_ENVIRON", "varValueFromEnv")
	dependency := Dependency{
		Repository:   "${REPO}",
		Branch:       "${NOBRANCHSET-$BRANCH}",
		ManifestPath: "${NOMPATHSET=$MPATH}",
		Namespace:    "${FOO+$SOME_NS_DEP_EXP}",
		Variables: env.Environment{
			env.Var{
				Name:  "MYVAR",
				Value: "${AVARVALUE}",
			},
			env.Var{
				Name:  "$${ANAME}",
				Value: "${MY_CUSTOM_VAR_FROM_ENVIRON}",
			},
		},
	}
	expected := Dependency{
		Repository:   "my/repo",
		Branch:       "myBranch",
		ManifestPath: "api/okteto.yml",
		Namespace:    "oktetoNs",
		Variables: env.Environment{
			env.Var{
				Name:  "MYVAR",
				Value: "thisIsAValue",
			},
			env.Var{
				Name:  "${ANAME}",
				Value: "varValueFromEnv",
			},
		},
	}
	envVariables := []string{
		"FOO=BAR",
		"REPO=my/repo",
		"BRANCH=myBranch",
		"MPATH=api/okteto.yml",
		"SOME_NS_DEP_EXP=oktetoNs",
		"AVARVALUE=thisIsAValue",
	}

	err := dependency.ExpandVars(envVariables)
	require.NoError(t, err)
	assert.Equal(t, expected, dependency)
}

func Test_ManifestDependencies_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		expected    ManifestSection
		name        string
		yaml        []byte
		expectedErr bool
	}{
		{
			name: "deserialized successfully from array",
			yaml: []byte(`
- https://github.com/okteto/movies-frontend
- https://github.com/okteto/movies-api`),
			expected: ManifestSection{
				"movies-api": &Dependency{
					Repository: "https://github.com/okteto/movies-api",
				},
				"movies-frontend": &Dependency{
					Repository: "https://github.com/okteto/movies-frontend",
				},
			},
		},
		{
			name: "deserialized successfully as map",
			yaml: []byte(`
frontend:
    repository: https://github.com/okteto/movies-frontend
    manifest: frontend.yml
    # ignored comment
    branch: frontend-branch
    variables:
      ENVIRONMENT: test
    wait: true
    timeout: 5m`),
			expected: ManifestSection{
				"frontend": &Dependency{
					Repository:   "https://github.com/okteto/movies-frontend",
					ManifestPath: "frontend.yml",
					Branch:       "frontend-branch",
					Variables: env.Environment{
						env.Var{
							Name:  "ENVIRONMENT",
							Value: "test",
						},
					},
					Wait:    true,
					Timeout: 5 * time.Minute,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var deps ManifestSection
			err := yaml.Unmarshal(tt.yaml, &deps)
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, deps)
			}
		})
	}
}

func Test_getRepoNameFromGitURL(t *testing.T) {
	tests := []struct {
		name     string
		repoUrl  string
		expected string
		wantErr  bool
	}{
		{
			name:     "https url with trailing slash",
			repoUrl:  "https://git-server.com/org/repo/",
			expected: "repo",
		},
		{
			name:     "https url",
			repoUrl:  "https://git-server.com/org/repo",
			expected: "repo",
		},
		{
			name:     "https url with git extension",
			repoUrl:  "https://git-server.com/org/repo.git",
			expected: "repo",
		},
		{
			name:     "ssh url",
			repoUrl:  "git@git-server.com:org/repo.git",
			expected: "repo",
		},
		{
			name:     "ssh url without .git",
			repoUrl:  "git@git-server.com:org/repo",
			expected: "repo",
		},
		{
			name:     "missing repo name",
			repoUrl:  "https//git-server/org",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "invalid url",
			repoUrl:  "foo//foo/foo",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "empty",
			repoUrl:  "foo",
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := giturls.Parse(tt.repoUrl)
			assert.NoError(t, err)
			result, err := getRepoNameFromGitURL(url)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}
