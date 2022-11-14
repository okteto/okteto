// Copyright 2022 The Okteto Authors
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
	"reflect"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
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

	p := &fakeProxy{}
	e := &fakeExecutor{
		err: assert.AnError,
	}
	dc := &DeployCommand{
		GetManifest:       getFakeManifest,
		Proxy:             p,
		Executor:          e,
		Kubeconfig:        &fakeKubeConfig{},
		K8sClientProvider: test.NewFakeK8sProvider(),
	}

	ctx := context.Background()

	fakeClient, _, err := dc.K8sClientProvider.Provide(clientcmdapi.NewConfig())
	if err != nil {
		t.Fatal("could not create fake k8s client")
	}

	expectedCfg := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "okteto-git-Name",
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

	currentCfg, err := getConfigMapFromData(ctx, data, fakeClient)
	if err != nil {
		t.Fatal("error trying to get configmap from data object")
	}

	assert.Equal(t, expectedCfg, currentCfg)
}

func Test_mergeServicesToDeployFromOptionsAndManifest(t *testing.T) {
	tests := []struct {
		name             string
		options          *Options
		expectedServices []string
	}{
		{
			name: "no manifest services to deploy",
			options: &Options{
				servicesToDeploy: []string{"a", "b"},
				Manifest: &model.Manifest{
					Deploy: &model.DeployInfo{
						ComposeSection: &model.ComposeSectionInfo{
							ComposesInfo: []model.ComposeInfo{},
						},
					},
				},
			},
			expectedServices: []string{"a", "b"},
		},
		{
			name: "no options services to deploy",
			options: &Options{
				Manifest: &model.Manifest{
					Deploy: &model.DeployInfo{
						ComposeSection: &model.ComposeSectionInfo{
							ComposesInfo: []model.ComposeInfo{
								{ServicesToDeploy: []string{"a", "b"}},
								{ServicesToDeploy: []string{"c", "d"}},
							},
						},
					},
				},
			},
			expectedServices: []string{"a", "b", "c", "d"},
		},
		{
			name: "both",
			options: &Options{
				servicesToDeploy: []string{"from command a", "from command b"},
				Manifest: &model.Manifest{
					Deploy: &model.DeployInfo{
						ComposeSection: &model.ComposeSectionInfo{
							ComposesInfo: []model.ComposeInfo{
								{ServicesToDeploy: []string{"c", "d"}},
							},
						},
					},
				},
			},
			expectedServices: []string{"from command a", "from command b"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mergeServicesToDeployFromOptionsAndManifest(test.options)
			// We have to check them as if they were sets to account for order
			expected := map[string]bool{}
			for _, service := range test.expectedServices {
				expected[service] = true
			}

			got := map[string]bool{}
			for _, service := range test.options.servicesToDeploy {
				got[service] = true
			}

			if !reflect.DeepEqual(expected, got) {
				t.Errorf("expected %v, got %v", expected, got)
			}
		})
	}
}

func Test_switchSSHRepoToHTTPS(t *testing.T) {
	tests := []struct {
		name              string
		repo              string
		expectedUrlString string
		expectedErr       error
	}{
		{
			name:              "input-ssh",
			repo:              "git@github.com:okteto/go-getting-started.git",
			expectedUrlString: "https://github.com/okteto/go-getting-started.git",
		},
		{
			name:              "input-https",
			repo:              "https://github.com/okteto/go-getting-started.git",
			expectedUrlString: "https://github.com/okteto/go-getting-started.git",
		},
		{
			name:              "input-http",
			repo:              "http://github.com/okteto/go-getting-started.git",
			expectedUrlString: "https://github.com/okteto/go-getting-started.git",
		},
		{
			name:        "input-not-allowed",
			repo:        "github.com/okteto/go-getting-started.git",
			expectedErr: assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := switchSSHRepoToHTTPS(tt.repo)
			if tt.expectedErr != nil && err == nil {
				t.Fatalf("expected err: %v but got no error", tt.expectedErr)
			}
			if tt.expectedErr == nil && err != nil {
				t.Fatalf("expected no err, but got: %v", err)
			}
			if tt.expectedErr != nil {
				assert.Error(t, err)
			}

			assert.Equal(t, tt.expectedUrlString, url.String())
		})
	}
}
