// Copyright 2025 The Okteto Authors
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

package up

import (
	"context"
	"errors"
	"testing"

	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

type MockApp struct {
	mock.Mock
	apps.App
}

func (m *MockApp) Deploy(ctx context.Context, k8sClient kubernetes.Interface) error {
	args := m.Called(ctx, k8sClient)
	return args.Error(0)
}

func (m *MockApp) ObjectMeta() metav1.ObjectMeta {
	args := m.Called()
	return args.Get(0).(metav1.ObjectMeta)
}

func (m *MockApp) TemplateObjectMeta() metav1.ObjectMeta {
	args := m.Called()
	return args.Get(0).(metav1.ObjectMeta)
}

func TestDevDeployer_DeployMainDev_Success(t *testing.T) {
	ctx := context.Background()
	k8sClient := fake.NewSimpleClientset()

	devApp := &MockApp{}
	devApp.On("Deploy", ctx, k8sClient).Return(nil)
	devApp.On("ObjectMeta").Return(metav1.ObjectMeta{
		Annotations: map[string]string{
			model.DeploymentRevisionAnnotation: "1",
		},
	})

	d := &model.Dev{
		Name: "main",
	}
	translations := map[string]*apps.Translation{
		"main": {
			MainDev: d,
			Dev:     d,
			DevApp:  devApp,
			App:     devApp,
		},
	}
	deployer := newDevDeployer(translations, k8sClient)

	err := deployer.deployMainDev(ctx)
	assert.NoError(t, err)

	devApp.AssertExpectations(t)
}

func TestDevDeployer_DeployMainDev_DeployError(t *testing.T) {
	ctx := context.Background()
	k8sClient := fake.NewSimpleClientset()

	devApp := &MockApp{}
	devApp.On("Deploy", ctx, k8sClient).Return(errors.New("deploy error"))
	devApp.On("ObjectMeta").Return(metav1.ObjectMeta{
		Annotations: map[string]string{
			model.DeploymentRevisionAnnotation: "1",
		},
	})

	d := &model.Dev{
		Name: "main",
	}

	translations := map[string]*apps.Translation{
		"main": {
			MainDev: d,
			Dev:     d,
			DevApp:  devApp,
			App:     devApp,
		},
	}

	deployer := newDevDeployer(translations, k8sClient)

	err := deployer.deployMainDev(ctx)
	assert.Error(t, err)

	devApp.AssertExpectations(t)
}

func TestDevDeployer_DeployDevServices_Success(t *testing.T) {
	ctx := context.Background()
	k8sClient := fake.NewSimpleClientset()

	devApp := &MockApp{}
	devApp.On("Deploy", ctx, k8sClient).Return(nil)
	devApp.On("ObjectMeta").Return(metav1.ObjectMeta{
		Annotations: map[string]string{
			model.DeploymentRevisionAnnotation: "1",
		},
	})
	devApp.On("TemplateObjectMeta").Return(metav1.ObjectMeta{
		Annotations: map[string]string{},
	})

	app := &MockApp{}
	app.On("Deploy", ctx, k8sClient).Return(nil)

	translations := map[string]*apps.Translation{
		"svc1": {
			MainDev: &model.Dev{
				Name: "main",
			},
			Dev: &model.Dev{
				Name: "svc1",
			},
			DevApp: devApp,
			App:    app,
		},
	}

	deployer := newDevDeployer(translations, k8sClient)
	deployer.servicesUpWait = true

	err := deployer.deployDevServices(ctx)
	assert.NoError(t, err)

	devApp.AssertExpectations(t)
	app.AssertExpectations(t)
}

func TestDevDeployer_DeployDevServices_DeployError(t *testing.T) {
	ctx := context.Background()
	k8sClient := fake.NewSimpleClientset()

	devApp := &MockApp{}
	devApp.On("Deploy", ctx, k8sClient).Return(errors.New("deploy error"))
	devApp.On("ObjectMeta").Return(metav1.ObjectMeta{
		Annotations: map[string]string{
			model.DeploymentRevisionAnnotation: "1",
		},
	})
	devApp.On("TemplateObjectMeta").Return(metav1.ObjectMeta{
		Annotations: map[string]string{},
	})

	translations := map[string]*apps.Translation{
		"svc1": {
			MainDev: &model.Dev{
				Name: "main",
			},
			Dev: &model.Dev{
				Name: "svc1",
			},
			DevApp: devApp,
			App:    &MockApp{},
		},
	}

	deployer := newDevDeployer(translations, k8sClient)
	deployer.servicesUpWait = true

	err := deployer.deployDevServices(ctx)
	assert.Error(t, err)

	devApp.AssertExpectations(t)
}
