// Copyright 2024 The Okteto Authors
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

package args

import (
	"context"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestNewDevModeOnLister(t *testing.T) {
	k8sClientProvider := test.NewFakeK8sProvider()
	lister := NewDevModeOnLister(k8sClientProvider)

	assert.NotNil(t, lister)
	assert.IsType(t, &DevModeOnLister{}, lister)
}
func TestDevModeOnLister_List(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Cfg: &api.Config{},
			},
		},
		CurrentContext: "test",
	}
	devs := model.ManifestDevs{
		"dev1": &model.Dev{
			Name: "dev1",
		},
		"dev2": &model.Dev{
			Name: "dev2",
		},
		"dev3": &model.Dev{
			Name: "dev3",
		},
	}
	ns := "ns"
	objects := []runtime.Object{
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dev1",
				Namespace: "ns",
				Labels: map[string]string{
					constants.DevLabel: "true",
				},
			},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dev2",
				Namespace: "ns",
				Labels: map[string]string{
					constants.DevLabel: "true",
				},
			},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dev3",
				Namespace: "ns",
			},
		},
	}
	provider := test.NewFakeK8sProvider(objects...)
	lister := NewDevModeOnLister(provider)

	devNameList, err := lister.List(context.Background(), devs, ns)

	assert.NoError(t, err)
	assert.Equal(t, []string{"dev1", "dev2"}, devNameList)
}

func TestManifestLister(t *testing.T) {
	devs := model.ManifestDevs{
		"dev1": &model.Dev{},
		"dev2": &model.Dev{},
		"dev3": &model.Dev{},
	}

	lister := NewManifestDevLister()

	devNameList, err := lister.List(context.Background(), devs, "")

	assert.NoError(t, err)
	assert.Equal(t, []string{"dev1", "dev2", "dev3"}, devNameList)
}
