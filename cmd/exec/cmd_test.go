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

package exec

import (
	"context"
	"fmt"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

type fakeMixpanelTracker struct {
}

func (f *fakeMixpanelTracker) Track(metadata *analytics.TrackExecMetadata) {}

type fakeExecutor struct {
	executionErr error
}

func (f *fakeExecutor) execute(ctx context.Context, cmd []string) error {
	return f.executionErr
}

type fakeExecutorProvider struct {
	executor executor
	err      error
}

func (f *fakeExecutorProvider) provide(*model.Dev, string) (executor, error) {
	return f.executor, f.err
}

func TestExec_Run(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Name: "test",
			},
		},
		CurrentContext: "test",
	}

	dev := &model.Dev{
		Name:      "test",
		Namespace: "test",
	}

	tt := []struct {
		expectedErr      error
		executorProvider executorProviderInterface
		name             string
		objects          []runtime.Object
		k8sClientErr     bool
	}{
		{
			name:        "error retrieving app",
			expectedErr: fmt.Errorf("development containers not found in namespace ''"),
		},
		{
			name:         "error providing kubernetes client",
			k8sClientErr: true,
			objects: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      dev.Name,
						Namespace: dev.Namespace,
						Labels: map[string]string{
							constants.DevLabel: "true",
						},
					},
				},
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      model.DevCloneName(dev.Name),
						Namespace: dev.Namespace,
						Labels: map[string]string{
							constants.DevLabel: "true",
						},
					},
				},
			},
			expectedErr: assert.AnError,
		},
		{
			name: "error getting pod running",
			objects: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      dev.Name,
						Namespace: dev.Namespace,
						Labels: map[string]string{
							constants.DevLabel: "true",
						},
						Annotations: map[string]string{
							model.DeploymentRevisionAnnotation: "test",
						},
						UID: "test",
					},
				},
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      model.DevCloneName(dev.Name),
						Namespace: dev.Namespace,
						Labels: map[string]string{
							constants.DevLabel: "true",
						},
					},
				},
				&appsv1.ReplicaSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      dev.Name,
						Namespace: dev.Namespace,
						Labels: map[string]string{
							constants.DevLabel: "true",
						},
						Annotations: map[string]string{
							model.DeploymentRevisionAnnotation: "test",
						},
						UID: "test",
						OwnerReferences: []metav1.OwnerReference{
							{
								UID: "test",
							},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      dev.Name,
						Namespace: dev.Namespace,
					},
				},
			},
			expectedErr: oktetoErrors.ErrNotFound,
		},
		{
			name: "error getting executor",
			objects: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      dev.Name,
						Namespace: dev.Namespace,
						Labels: map[string]string{
							constants.DevLabel: "true",
						},
						Annotations: map[string]string{
							model.DeploymentRevisionAnnotation: "test",
						},
						UID: "test",
					},
				},
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      model.DevCloneName(dev.Name),
						Namespace: dev.Namespace,
						Labels: map[string]string{
							constants.DevLabel: "true",
						},
						Annotations: map[string]string{
							model.DeploymentRevisionAnnotation: "test",
						},
						UID: "test",
					},
				},
				&appsv1.ReplicaSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      dev.Name,
						Namespace: dev.Namespace,
						Labels: map[string]string{
							constants.DevLabel: "true",
						},
						Annotations: map[string]string{
							model.DeploymentRevisionAnnotation: "test",
						},
						UID: "test",
						OwnerReferences: []metav1.OwnerReference{
							{
								UID: "test",
							},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      dev.Name,
						Namespace: dev.Namespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								UID: "test",
							},
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "test",
							},
						},
					},
				},
			},
			executorProvider: &fakeExecutorProvider{
				err: assert.AnError,
			},
			expectedErr: assert.AnError,
		},
		{
			name: "error executing",
			objects: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      dev.Name,
						Namespace: dev.Namespace,
						Labels: map[string]string{
							constants.DevLabel: "true",
						},
						Annotations: map[string]string{
							model.DeploymentRevisionAnnotation: "test",
						},
						UID: "test",
					},
				},
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      model.DevCloneName(dev.Name),
						Namespace: dev.Namespace,
						Labels: map[string]string{
							constants.DevLabel: "true",
						},
						Annotations: map[string]string{
							model.DeploymentRevisionAnnotation: "test",
						},
						UID: "test",
					},
				},
				&appsv1.ReplicaSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      dev.Name,
						Namespace: dev.Namespace,
						Labels: map[string]string{
							constants.DevLabel: "true",
						},
						Annotations: map[string]string{
							model.DeploymentRevisionAnnotation: "test",
						},
						UID: "test",
						OwnerReferences: []metav1.OwnerReference{
							{
								UID: "test",
							},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      dev.Name,
						Namespace: dev.Namespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								UID: "test",
							},
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "test",
							},
						},
					},
				},
			},
			executorProvider: &fakeExecutorProvider{
				executor: &fakeExecutor{
					executionErr: assert.AnError,
				},
				err: nil,
			},
			expectedErr: assert.AnError,
		},
		{
			name: "successful execution",
			objects: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      dev.Name,
						Namespace: dev.Namespace,
						Labels: map[string]string{
							constants.DevLabel: "true",
						},
						Annotations: map[string]string{
							model.DeploymentRevisionAnnotation: "test",
						},
						UID: "test",
					},
				},
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      model.DevCloneName(dev.Name),
						Namespace: dev.Namespace,
						Labels: map[string]string{
							constants.DevLabel: "true",
						},
						Annotations: map[string]string{
							model.DeploymentRevisionAnnotation: "test",
						},
						UID: "test",
					},
				},
				&appsv1.ReplicaSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      dev.Name,
						Namespace: dev.Namespace,
						Labels: map[string]string{
							constants.DevLabel: "true",
						},
						Annotations: map[string]string{
							model.DeploymentRevisionAnnotation: "test",
						},
						UID: "test",
						OwnerReferences: []metav1.OwnerReference{
							{
								UID: "test",
							},
						},
					},
				},
				&corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      dev.Name,
						Namespace: dev.Namespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								UID: "test",
							},
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "test",
							},
						},
					},
				},
			},
			executorProvider: &fakeExecutorProvider{
				executor: &fakeExecutor{},
				err:      nil,
			},
			expectedErr: nil,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			k8sClientProvider := test.NewFakeK8sProvider(tc.objects...)
			if tc.k8sClientErr {
				k8sClientProvider.ErrProvide = assert.AnError
			}
			k8sClientProviderForAppRetriever := test.NewFakeK8sProvider(tc.objects...)
			ioCtrl := io.NewIOController()
			appRetriever := newAppRetriever(ioCtrl, k8sClientProviderForAppRetriever)
			appRetriever.newRunningAppGetter = func(c kubernetes.Interface) *runningAppGetter {
				return &runningAppGetter{
					k8sClient: c,
				}
			}
			e := &Exec{
				ioCtrl:            ioCtrl,
				k8sClientProvider: k8sClientProvider,
				appRetriever:      appRetriever,
				mixpanelTracker:   &fakeMixpanelTracker{},
				executorProvider:  tc.executorProvider,
			}
			err := e.Run(
				context.Background(),
				&options{
					devName: "test",
					command: []string{"echo", "test"},
				},
				dev)
			if err != nil {
				assert.ErrorContains(t, err, tc.expectedErr.Error())
			} else {
				assert.NoError(t, tc.expectedErr)
			}
		})
	}
}
