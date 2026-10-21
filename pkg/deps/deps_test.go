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
			name: "deserialized successfully from array with repo with groups",
			yaml: []byte(`
- https://gitlab.com/okteto24/samples/nested/movies-frontend
- https://gitlab.com/okteto24/samples/nested/movies-api`),
			expected: ManifestSection{
				"movies-api": &Dependency{
					Repository: "https://gitlab.com/okteto24/samples/nested/movies-api",
				},
				"movies-frontend": &Dependency{
					Repository: "https://gitlab.com/okteto24/samples/nested/movies-frontend",
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

func Test_DependsOn_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		yaml     []byte
		expected DependsOn
	}{
		{
			name:     "single name",
			yaml:     []byte(`database`),
			expected: DependsOn{"database"},
		},
		{
			name:     "list of names",
			yaml:     []byte(`[database, messaging]`),
			expected: DependsOn{"database", "messaging"},
		},
		{
			name:     "empty list",
			yaml:     []byte(`[]`),
			expected: DependsOn{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dependsOn DependsOn
			require.NoError(t, yaml.Unmarshal(tt.yaml, &dependsOn))
			require.Equal(t, tt.expected, dependsOn)
		})
	}
}

func Test_ManifestSection_UnmarshalYAML_WithDependsOn(t *testing.T) {
	manifest := []byte(`
database:
  repository: https://github.com/okteto/database
backend:
  repository: https://github.com/okteto/backend
  depends_on: [database, messaging]
  wait: true
analytics:
  repository: https://github.com/okteto/analytics
  depends_on: database
messaging:
  repository: https://github.com/okteto/messaging`)

	var deps ManifestSection
	require.NoError(t, yaml.UnmarshalStrict(manifest, &deps))

	require.Equal(t, ManifestSection{
		"database":  &Dependency{Repository: "https://github.com/okteto/database"},
		"messaging": &Dependency{Repository: "https://github.com/okteto/messaging"},
		"backend": &Dependency{
			Repository: "https://github.com/okteto/backend",
			DependsOn:  DependsOn{"database", "messaging"},
			Wait:       true,
		},
		"analytics": &Dependency{
			Repository: "https://github.com/okteto/analytics",
			DependsOn:  DependsOn{"database"},
		},
	}, deps)
}

func Test_ManifestSection_UnmarshalYAML_DuplicatedNameInList(t *testing.T) {
	manifest := []byte(`
- https://github.com/okteto/movies-api
- https://gitlab.com/okteto/movies-api`)

	var deps ManifestSection
	err := yaml.Unmarshal(manifest, &deps)

	require.EqualError(t, err, "dependency 'movies-api' is defined more than once")
}

func Test_ManifestSection_UnmarshalYAML_DuplicatedNameInMap(t *testing.T) {
	manifest := []byte(`
database:
  repository: https://github.com/okteto/database
database:
  repository: https://github.com/okteto/other-database`)

	var deps ManifestSection
	err := yaml.UnmarshalStrict(manifest, &deps)

	require.Error(t, err)
}

func Test_ManifestSection_Validate(t *testing.T) {
	tests := []struct {
		deps ManifestSection
		name string
	}{
		{
			name: "no dependencies",
			deps: ManifestSection{},
		},
		{
			name: "dependencies without depends_on",
			deps: ManifestSection{
				"database":  &Dependency{Repository: "https://github.com/okteto/database"},
				"messaging": &Dependency{Repository: "https://github.com/okteto/messaging", DependsOn: DependsOn{}},
			},
		},
		{
			name: "diamond graph with forward references",
			deps: ManifestSection{
				"frontend":  &Dependency{Repository: "https://github.com/okteto/frontend", DependsOn: DependsOn{"backend", "analytics"}},
				"backend":   &Dependency{Repository: "https://github.com/okteto/backend", DependsOn: DependsOn{"database"}},
				"analytics": &Dependency{Repository: "https://github.com/okteto/analytics", DependsOn: DependsOn{"database"}},
				"database":  &Dependency{Repository: "https://github.com/okteto/database"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, tt.deps.Validate())
		})
	}
}

func Test_ManifestSection_Validate_NilDependency(t *testing.T) {
	deps := ManifestSection{
		"database": nil,
	}

	err := deps.Validate()

	require.EqualError(t, err, "manifest validation failed: dependency 'database' is not defined correctly")
}

func Test_ManifestSection_Validate_SelfDependency(t *testing.T) {
	deps := ManifestSection{
		"worker": &Dependency{Repository: "https://github.com/okteto/worker", DependsOn: DependsOn{"worker"}},
	}

	err := deps.Validate()

	require.EqualError(t, err, "manifest validation failed: dependency 'worker' cannot depend on itself")
}

func Test_ManifestSection_Validate_UnknownReference(t *testing.T) {
	deps := ManifestSection{
		"api": &Dependency{Repository: "https://github.com/okteto/api", DependsOn: DependsOn{"cache"}},
	}

	err := deps.Validate()

	require.EqualError(t, err, "manifest validation failed: dependency 'api' depends on 'cache', which is not defined in the dependencies section")
}

func Test_ManifestSection_Validate_Cycle(t *testing.T) {
	deps := ManifestSection{
		"a": &Dependency{Repository: "https://github.com/okteto/a", DependsOn: DependsOn{"c"}},
		"b": &Dependency{Repository: "https://github.com/okteto/b", DependsOn: DependsOn{"a"}},
		"c": &Dependency{Repository: "https://github.com/okteto/c", DependsOn: DependsOn{"b"}},
	}

	err := deps.Validate()

	require.EqualError(t, err, "manifest validation failed: cyclic dependency found between dependencies a, b and c")
}

func Test_ManifestSection_Validate_CycleOnlyNamesItsDependencies(t *testing.T) {
	deps := ManifestSection{
		"frontend": &Dependency{Repository: "https://github.com/okteto/frontend", DependsOn: DependsOn{"api"}},
		"api":      &Dependency{Repository: "https://github.com/okteto/api", DependsOn: DependsOn{"db"}},
		"db":       &Dependency{Repository: "https://github.com/okteto/db", DependsOn: DependsOn{"cache"}},
		"cache":    &Dependency{Repository: "https://github.com/okteto/cache", DependsOn: DependsOn{"db"}},
	}

	err := deps.Validate()

	require.EqualError(t, err, "manifest validation failed: cyclic dependency found between dependencies cache and db")
}

func Test_ExpandVarsFrom_UsesTheGivenEnviron(t *testing.T) {
	t.Setenv("DEPS_TEST_BRANCH", "from-process")
	dependency := &Dependency{Branch: "${DEPS_TEST_BRANCH}"}

	err := dependency.ExpandVarsFrom([]string{"DEPS_TEST_BRANCH=from-snapshot"}, nil)

	require.NoError(t, err)
	require.Equal(t, "from-snapshot", dependency.Branch)
}

func Test_ManifestSection_Validate_NamesResolvingToTheSamePipeline(t *testing.T) {
	deps := ManifestSection{
		"my.api": &Dependency{Repository: "https://github.com/okteto/api"},
		"my-api": &Dependency{Repository: "https://github.com/okteto/other-api"},
	}

	err := deps.Validate()

	require.EqualError(t, err, "manifest validation failed: dependencies 'my-api' and 'my.api' resolve to the same name 'my-api'")
}

func Test_Dependency_UnmarshalYAML_DoesNotExpandVariables(t *testing.T) {
	t.Setenv("DEPS_TEST_HOST", "at-read")
	manifest := []byte(`
backend:
  repository: https://github.com/okteto/backend
  variables:
    DB_HOST: ${DEPS_TEST_HOST}
    ESCAPED: $${DEPS_TEST_HOST}
`)

	var section ManifestSection
	require.NoError(t, yaml.Unmarshal(manifest, &section))

	require.Equal(t, env.Environment{
		{Name: "DB_HOST", Value: "${DEPS_TEST_HOST}"},
		{Name: "ESCAPED", Value: "$${DEPS_TEST_HOST}"},
	}, section["backend"].Variables)
}

func Test_Dependency_UnmarshalYAML_DoesNotExpandVariablesInListForm(t *testing.T) {
	t.Setenv("DEPS_TEST_HOST", "at-read")
	manifest := []byte(`
backend:
  repository: https://github.com/okteto/backend
  variables:
    - DB_HOST=${DEPS_TEST_HOST}
    - DEPS_TEST_HOST
`)

	var section ManifestSection
	require.NoError(t, yaml.Unmarshal(manifest, &section))

	// a variable without value still takes it from the environment
	require.Equal(t, env.Environment{
		{Name: "DB_HOST", Value: "${DEPS_TEST_HOST}"},
		{Name: "DEPS_TEST_HOST", Value: "at-read"},
	}, section["backend"].Variables)
}

func Test_ExpandVarsFrom_ExpandsVariablesTwice(t *testing.T) {
	dependency := &Dependency{
		Variables: env.Environment{
			{Name: "NATURAL", Value: "${DEPS_TEST_HOST}"},
			{Name: "ESCAPED", Value: "$${DEPS_TEST_HOST}"},
			{Name: "UNDEFINED", Value: "${DEPS_TEST_UNDEFINED}"},
			{Name: "ESCAPED_UNDEFINED", Value: "$${DEPS_TEST_UNDEFINED}"},
			{Name: "LITERAL", Value: "value"},
		},
	}

	err := dependency.ExpandVarsFrom([]string{"DEPS_TEST_HOST=db.svc"}, nil)

	require.NoError(t, err)
	// the same results as expanding them when the manifest is read and again when deploying
	require.Equal(t, env.Environment{
		{Name: "NATURAL", Value: "db.svc"},
		{Name: "ESCAPED", Value: "db.svc"},
		{Name: "UNDEFINED", Value: ""},
		{Name: "ESCAPED_UNDEFINED", Value: "${DEPS_TEST_UNDEFINED}"},
		{Name: "LITERAL", Value: "value"},
	}, dependency.Variables)
}
