// Copyright 2020 The Okteto Authors
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
package cmd

import (
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/pkg/model"
)

func Test_overwriteFieldsWithArgs(t *testing.T) {
	tests := []struct {
		name       string
		dev        *model.Dev
		path       string
		dockerfile string
		target     string
		want       *model.BuildInfoRaw
	}{
		{
			name: "empty",
			dev: &model.Dev{
				Build: &model.BuildInfo{
					BuildInfoRaw: model.BuildInfoRaw{
						Context:    ".",
						Dockerfile: filepath.Join(".", "Dockerfile"),
					},
				},
			},
			path:       "",
			dockerfile: "",
			target:     "",
			want: &model.BuildInfoRaw{
				Context:    ".",
				Dockerfile: "Dockerfile",
			},
		},
		{
			name: "context",
			dev: &model.Dev{
				Build: &model.BuildInfo{
					BuildInfoRaw: model.BuildInfoRaw{
						Context:    ".",
						Dockerfile: filepath.Join(".", "Dockerfile"),
					},
				},
			},
			path:       "context",
			dockerfile: "",
			target:     "",
			want: &model.BuildInfoRaw{
				Context:    "context",
				Dockerfile: filepath.Join("context", "Dockerfile"),
			},
		},
		{
			name: "dockerfile",
			dev: &model.Dev{
				Build: &model.BuildInfo{
					BuildInfoRaw: model.BuildInfoRaw{
						Context:    ".",
						Dockerfile: filepath.Join(".", "Dockerfile"),
					},
				},
			},
			path:       "",
			dockerfile: "dockerfile",
			target:     "",
			want: &model.BuildInfoRaw{
				Context:    ".",
				Dockerfile: "dockerfile",
			},
		},
		{
			name: "target",
			dev: &model.Dev{
				Build: &model.BuildInfo{
					BuildInfoRaw: model.BuildInfoRaw{
						Context:    ".",
						Dockerfile: filepath.Join(".", "Dockerfile"),
					},
				},
			},
			path:       "",
			dockerfile: "",
			target:     "target",
			want: &model.BuildInfoRaw{
				Context:    ".",
				Dockerfile: "Dockerfile",
				Target:     "target",
			},
		},
		{
			name: "args",
			dev: &model.Dev{
				Build: &model.BuildInfo{
					BuildInfoRaw: model.BuildInfoRaw{
						Context:    ".",
						Dockerfile: filepath.Join(".", "Dockerfile"),
					},
				},
			},
			path:       "",
			dockerfile: "",
			target:     "",
			want: &model.BuildInfoRaw{
				Context:    ".",
				Dockerfile: "Dockerfile",
				Args: []model.EnvVar{
					{
						Name:  "a",
						Value: "1",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			overwriteFieldsWithArgs(tt.dev, tt.path, tt.dockerfile, "", tt.target)
			if tt.want.Context != tt.dev.Build.Context {
				t.Errorf("overwriteFieldsWithArgs '%s' got path %s, want %s", tt.name, tt.dev.Build.Context, tt.want.Context)
			}
			if tt.want.Dockerfile != tt.dev.Build.Dockerfile {
				t.Errorf("overwriteFieldsWithArgs '%s' got dockerfile %s, want %s", tt.name, tt.dev.Build.Dockerfile, tt.want.Dockerfile)
			}
			if tt.want.Target != tt.dev.Build.Target {
				t.Errorf("overwriteFieldsWithArgs '%s' got target %s, want %s", tt.name, tt.dev.Build.Target, tt.want.Target)
			}
		})
	}
}
