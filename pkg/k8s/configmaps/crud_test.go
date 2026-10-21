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

package configmaps_test

import (
	"context"
	"errors"
	"testing"
	"time"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8sTesting "k8s.io/client-go/testing"
)

func TestGet(t *testing.T) {
	ctx := context.Background()
	ns := "fake-ns"
	mockCfgName := "fake-cfg"
	mockCfgStatus := "fake-status"
	mockCfg := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mockCfgName,
			Namespace: ns,
		},
		Data: map[string]string{
			"status": mockCfgStatus,
		},
	}
	clientset := fake.NewSimpleClientset(mockCfg)

	cfg, err := configmaps.Get(ctx, mockCfgName, ns, clientset)

	require.NoError(t, err)
	assert.Equal(t, mockCfg, cfg)
}

func TestGet_Err(t *testing.T) {
	ctx := context.Background()

	mockErr := errors.New("unexpected error")

	clientset := fake.NewSimpleClientset()
	clientset.Fake.PrependReactor("get", "configmaps", func(action k8sTesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, mockErr
	})

	cfg, err := configmaps.Get(ctx, "fake-cfg-name", "fake-ns", clientset)

	assert.Nil(t, cfg)
	assert.ErrorIs(t, err, mockErr)
}

func TestWaitForStatus(t *testing.T) {
	ctx := context.Background()

	ticker := time.NewTicker(1 * time.Millisecond)

	mockCfgName := "fake-cfg"
	ns := "fake-ns"

	mockCfg := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mockCfgName,
			Namespace: ns,
		},
		Data: map[string]string{
			"status": "fake-initial-status",
		},
	}

	clientset := fake.NewSimpleClientset(mockCfg)

	go func() {
		mockCfg.Data["status"] = "fake-final-status"
		_, err := clientset.CoreV1().ConfigMaps(ns).Update(ctx, mockCfg, metav1.UpdateOptions{})
		if err != nil {
			t.Fail()
		}
	}()

	err := configmaps.WaitForStatus(ctx, mockCfgName, ns, "fake-final-status", ticker, 5*time.Millisecond, clientset)
	assert.Nil(t, err)
}

func TestWaitForStatus_Timeout(t *testing.T) {
	ctx := context.Background()

	ticker := time.NewTicker(100 * time.Millisecond)

	mockCfgName := "fake-cfg"
	ns := "fake-ns"

	mockCfg := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mockCfgName,
			Namespace: ns,
		},
		Data: map[string]string{
			"status": "fake-initial-status",
		},
	}

	clientset := fake.NewSimpleClientset(mockCfg)
	err := configmaps.WaitForStatus(ctx, mockCfgName, ns, "fake-final-status", ticker, 1*time.Microsecond, clientset)
	assert.ErrorIs(t, err, oktetoErrors.ErrTimeout)
}

func TestWaitForStatus_NotFound(t *testing.T) {
	ctx := context.Background()

	ticker := time.NewTicker(1 * time.Millisecond)

	clientset := fake.NewSimpleClientset()
	clientset.Fake.PrependReactor("get", "configmaps", func(action k8sTesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, k8sErrors.NewNotFound(apiv1.Resource("configmaps"), "fake-cfg")
	})

	err := configmaps.WaitForStatus(ctx, "fake-cfg", "fake-ns", "fake-final-status", ticker, 5*time.Second, clientset)
	assert.ErrorIs(t, err, oktetoErrors.ErrNotFound)
}

func TestList(t *testing.T) {
	ctx := context.Background()
	ns := "fake-ns"
	mockCfgStatus := "fake-status"
	labelSelector := "app=my-app"

	fakeCm1 := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-cm-1",
			Namespace: ns,
			Labels:    map[string]string{"app": "my-app"},
		},
		Data: map[string]string{
			"status": mockCfgStatus,
		},
	}

	fakeCm2 := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-cm-2",
			Namespace: ns,
			Labels:    map[string]string{"app": "my-app"},
		},
		Data: map[string]string{
			"status": mockCfgStatus,
		},
	}

	clientset := fake.NewSimpleClientset(fakeCm1, fakeCm2)

	cfgs, err := configmaps.List(ctx, ns, labelSelector, clientset)

	require.NoError(t, err)
	assert.Len(t, cfgs, 2)
	assert.Contains(t, cfgs, *fakeCm1)
	assert.Contains(t, cfgs, *fakeCm2)
}

func TestList_Err(t *testing.T) {
	ctx := context.Background()

	mockErr := assert.AnError

	clientset := fake.NewSimpleClientset()
	clientset.Fake.PrependReactor("list", "configmaps", func(action k8sTesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, mockErr
	})

	cfg, err := configmaps.List(ctx, "fake-cfg-name", "fake-ns", clientset)

	assert.Nil(t, cfg)
	assert.ErrorIs(t, err, mockErr)
}
