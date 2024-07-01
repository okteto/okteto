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

package deploy

import (
	"context"
	"net/url"
	"reflect"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetConfigMapFromData(t *testing.T) {
	manifest := []byte(`icon: https://apps.okteto.com/movies/icon.png
deploy:
    - okteto build -t okteto.dev/api:${OKTETO_GIT_COMMIT} api
    - okteto build -t okteto.dev/frontend:${OKTETO_GIT_COMMIT} frontend
    - helm upgrade --install movies chart --set tag=${OKTETO_GIT_COMMIT}
devs:
    - api/okteto.yml
    - frontend/okteto.yml`)
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
			},
		},
		CurrentContext: "test",
	}
	data := &pipeline.CfgData{
		Name:       "Name",
		Namespace:  "Namespace",
		Repository: "https://github.com/okteto/movies",
		Branch:     "master",
		Filename:   "Filename",
		Status:     "progressing",
		Manifest:   manifest,
		Icon:       "https://apps.okteto.com/movies/icon.png",
	}

	fakeK8sProvider := test.NewFakeK8sProvider()
	dc := &Command{
		GetManifest:       getFakeManifest,
		K8sClientProvider: fakeK8sProvider,
		CfgMapHandler:     newDefaultConfigMapHandler(fakeK8sProvider, nil),
	}

	ctx := context.Background()

	expectedCfg := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "okteto-git-name",
			Namespace: "Namespace",
			Labels:    map[string]string{"dev.okteto.com/git-deploy": "true"},
		},
		Data: map[string]string{
			"actionName": "cli",
			"branch":     "master",
			"filename":   "Filename",
			"icon":       "https://apps.okteto.com/movies/icon.png",
			"name":       "Name",
			"output":     "",
			"repository": "https://github.com/okteto/movies",
			"status":     "progressing",
			"yaml":       "aWNvbjogaHR0cHM6Ly9hcHBzLm9rdGV0by5jb20vbW92aWVzL2ljb24ucG5nCmRlcGxveToKICAgIC0gb2t0ZXRvIGJ1aWxkIC10IG9rdGV0by5kZXYvYXBpOiR7T0tURVRPX0dJVF9DT01NSVR9IGFwaQogICAgLSBva3RldG8gYnVpbGQgLXQgb2t0ZXRvLmRldi9mcm9udGVuZDoke09LVEVUT19HSVRfQ09NTUlUfSBmcm9udGVuZAogICAgLSBoZWxtIHVwZ3JhZGUgLS1pbnN0YWxsIG1vdmllcyBjaGFydCAtLXNldCB0YWc9JHtPS1RFVE9fR0lUX0NPTU1JVH0KZGV2czoKICAgIC0gYXBpL29rdGV0by55bWwKICAgIC0gZnJvbnRlbmQvb2t0ZXRvLnltbA==",
		},
	}

	currentCfg, err := dc.CfgMapHandler.TranslateConfigMapAndDeploy(ctx, data)
	if err != nil {
		t.Fatal("error trying to get configmap from data object")
	}

	assert.Equal(t, expectedCfg.Name, currentCfg.Name)
	assert.Equal(t, expectedCfg.Namespace, currentCfg.Namespace)
	assert.Equal(t, expectedCfg.Labels, currentCfg.Labels)
	assert.Equal(t, expectedCfg.Data, currentCfg.Data)
	assert.NotEmpty(t, currentCfg.Annotations[constants.LastUpdatedAnnotation])
}

func Test_switchSSHRepoToHTTPS(t *testing.T) {
	tests := []struct {
		expected *url.URL
		name     string
		repo     string
	}{
		{
			name: "input-ssh",
			repo: "git@github.com:okteto/go-getting-started.git",
			expected: &url.URL{
				Scheme: "https",
				Host:   "github.com",
				Path:   "okteto/go-getting-started.git",
			},
		},
		{
			name: "input-https",
			repo: "https://github.com/okteto/go-getting-started.git",
			expected: &url.URL{
				Scheme: "https",
				Host:   "github.com",
				Path:   "/okteto/go-getting-started.git",
			}},
		{
			name: "input-http",
			repo: "http://github.com/okteto/go-getting-started.git",
			expected: &url.URL{
				Scheme: "https",
				Host:   "github.com",
				Path:   "/okteto/go-getting-started.git",
			}},
		{
			name:     "input-local",
			repo:     "github.com/okteto/go-getting-started.git",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := switchRepoSchemaToHTTPS(tt.repo)
			assert.Equal(t, tt.expected, url)
		})
	}
}
func Test_getStackServicesToDeploy(t *testing.T) {
	stack := &model.Stack{
		Name: "test-stack",
		Services: map[string]*model.Service{
			"service1": {},
			"service2": {},
			"service3": {},
			"service4": {},
		},
	}
	tests := []struct {
		name               string
		composeSectionInfo *model.ComposeSectionInfo
		expected           []string
	}{
		{
			name: "MultipleComposeInfo",
			composeSectionInfo: &model.ComposeSectionInfo{
				ComposesInfo: []model.ComposeInfo{
					{
						ServicesToDeploy: []string{"service1", "service2"},
					},
					{
						ServicesToDeploy: []string{"service3", "service4"},
					},
				},
				Stack: stack,
			},
			expected: []string{"service1", "service2", "service3", "service4"},
		},
		{
			name: "MultipleComposeInfo",
			composeSectionInfo: &model.ComposeSectionInfo{
				ComposesInfo: []model.ComposeInfo{
					{
						ServicesToDeploy: []string{"service1", "service2"},
					},
					{
						ServicesToDeploy: []string{"nonexistent", "service4"},
					},
				},
				Stack: stack,
			},
			expected: []string{},
		},
		{
			name: "SingleComposeInfo",
			composeSectionInfo: &model.ComposeSectionInfo{
				ComposesInfo: []model.ComposeInfo{
					{
						ServicesToDeploy: []string{"service1", "service2", "service3"},
					},
				},
				Stack: stack,
			},
			expected: []string{"service1", "service2", "service3"},
		},
		{
			name: "EmptyComposeInfo",
			composeSectionInfo: &model.ComposeSectionInfo{
				ComposesInfo: []model.ComposeInfo{},
				Stack:        stack,
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			c := fake.NewSimpleClientset()

			svcs, err := getStackServicesToDeploy(ctx, tt.composeSectionInfo, c)
			if err != nil {
				t.Fatalf("failed to get stack services to deploy: %s", err)
			}

			if !reflect.DeepEqual(svcs, tt.expected) {
				t.Errorf("got stack services to deploy %v, expected %v", svcs, tt.expected)
			}
		})
	}
}
