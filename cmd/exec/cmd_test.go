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

func (f *fakeMixpanelTracker) Track()                                            {}
func (f *fakeMixpanelTracker) SetMetadata(metadata *analytics.TrackExecMetadata) {}

type fakeExecutor struct {
	executionErr error
}

func (f *fakeExecutor) execute(ctx context.Context, cmd []string) error {
	return f.executionErr
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
		name            string
		objects         []runtime.Object
		k8sClientErr    bool
		getExecutorFunc func(dev *model.Dev, podName string) (executor, error)
		expectedErr     error
	}{
		{
			name:        "error retrieving app",
			expectedErr: fmt.Errorf("the application 'test' referred by your okteto manifest doesn't exist"),
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
			getExecutorFunc: func(dev *model.Dev, podName string) (executor, error) {
				return nil, assert.AnError
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
			getExecutorFunc: func(dev *model.Dev, podName string) (executor, error) {
				return &fakeExecutor{
					executionErr: assert.AnError,
				}, nil
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
			getExecutorFunc: func(dev *model.Dev, podName string) (executor, error) {
				return &fakeExecutor{}, nil
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
				mixpannelTracker:  &fakeMixpanelTracker{},
				getExecutorFunc:   tc.getExecutorFunc,
			}
			err := e.Run(
				context.Background(),
				&Options{
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
