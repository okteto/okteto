package up

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/model"
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

	err := deployer.deployDevServices(ctx)
	assert.Error(t, err)

	devApp.AssertExpectations(t)
}
