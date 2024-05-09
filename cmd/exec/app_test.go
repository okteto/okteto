package exec

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestWaitUnitlDevModeIsReady(t *testing.T) {
	dev := &model.Dev{}

	tt := []struct {
		name        string
		waiter      func(*model.Dev, []config.UpState) error
		expectedErr error
	}{
		{
			name: "error",
			waiter: func(dev *model.Dev, states []config.UpState) error {
				return assert.AnError
			},
			expectedErr: assert.AnError,
		},
		{
			name: "success",
			waiter: func(dev *model.Dev, states []config.UpState) error {
				return nil
			},
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			w := waitUnitlDevModeIsReady{
				statusWaiter: tc.waiter,
			}
			err := w.Wait(dev, nil)
			assert.ErrorIs(t, err, tc.expectedErr)
		})
	}
}

func TestWaitUntilAppIsInDevMode_Wait(t *testing.T) {
	dev := &model.Dev{}
	tt := []struct {
		name        string
		dev         bool
		expectedErr error
	}{
		{
			name: "dev mode enabled",
			dev:  true,
		},
		{
			name: "dev mode not enabled",
			dev:  false,
			expectedErr: oktetoErrors.UserError{
				E:    fmt.Errorf("development mode is not enabled"),
				Hint: "Run 'okteto up' to enable it and try again",
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			w := waitUntilAppIsInDevMode{}
			app := apps.NewDeploymentApp(&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						constants.DevLabel: strconv.FormatBool(tc.dev),
					},
				},
			})
			err := w.Wait(dev, app)
			if tc.expectedErr != nil {
				assert.ErrorContains(t, err, tc.expectedErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

type fakeWaiter struct {
	err error
}

func (f fakeWaiter) Wait(*model.Dev, apps.App) error {
	return f.err
}

func TestRunningAppGetter_GetApp(t *testing.T) {
	dev := &model.Dev{
		Name:      "test",
		Namespace: "test",
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dev.Name,
			Namespace: dev.Namespace,
		},
	}
	tt := []struct {
		name        string
		objects     []runtime.Object
		mockWaiters []waiter
		expectedErr error
	}{
		{
			name: "GetApp success",
			objects: []runtime.Object{
				deployment,
			},
			mockWaiters: []waiter{
				fakeWaiter{},
			},
			expectedErr: nil,
		},
		{
			name:        "GetApp error",
			expectedErr: errors.New("get app error"),
			mockWaiters: []waiter{
				fakeWaiter{},
			},
		},
		{
			name: "Waiter error",
			objects: []runtime.Object{
				deployment,
			},
			expectedErr: errors.New("waiter error"),
			mockWaiters: []waiter{
				fakeWaiter{assert.AnError},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			c, _, _ := test.NewFakeK8sProvider(tc.objects...).Provide(nil)

			r := &runningAppGetter{
				waiter:    tc.mockWaiters,
				k8sClient: c,
			}
			_, err := r.GetApp(context.Background(), dev)
			if tc.expectedErr != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
func TestNewRunningAppGetter(t *testing.T) {
	c, _, _ := test.NewFakeK8sProvider().Provide(nil)
	g := newRunningAppGetter(c)

	assert.NotNil(t, g)
	assert.Equal(t, c, g.k8sClient)
	assert.Len(t, g.waiter, 2)
	assert.IsType(t, waitUntilAppIsInDevMode{}, g.waiter[0])
	assert.IsType(t, waitUnitlDevModeIsReady{}, g.waiter[1])
}
