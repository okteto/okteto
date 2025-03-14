package waitfor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestWaitForDeployment(t *testing.T) {
	tests := []struct {
		name         string
		setupMock    func(kubernetes.Interface, string, string)
		resourceName string
		condition    model.DependsOnCondition
		timeout      time.Duration
		expectError  bool
	}{
		{
			name: "deployment ready - service_started",
			setupMock: func(c kubernetes.Interface, ns, name string) {
				deployments.Deploy(
					context.Background(),
					&appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: ns,
						},
						Status: appsv1.DeploymentStatus{
							ReadyReplicas: 1,
						},
					},
					c)
			},
			resourceName: "ready-deployment",
			condition:    model.DependsOnServiceRunning,
			timeout:      3 * time.Second,
			expectError:  false,
		},
		{
			name: "deployment healthy - service_healthy",
			setupMock: func(c kubernetes.Interface, ns, name string) {
				deployments.Deploy(
					context.Background(),
					&appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: ns,
						},
						Status: appsv1.DeploymentStatus{
							ReadyReplicas: 1,
						},
					},
					c)
				c.CoreV1().Endpoints(ns).Create(context.Background(), &corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: ns,
					},
					Subsets: []corev1.EndpointSubset{
						{
							Addresses: []corev1.EndpointAddress{},
							Ports:     []corev1.EndpointPort{},
						},
					},
				}, metav1.CreateOptions{})
			},
			resourceName: "healthy-deployment",
			condition:    model.DependsOnServiceHealthy,
			timeout:      3 * time.Second,
			expectError:  false,
		},
		{
			name: "deployment timeout",
			setupMock: func(c kubernetes.Interface, ns, name string) {
				// intentionally empty to simulate timeout
			},
			resourceName: "missing-deployment",
			condition:    model.DependsOnServiceRunning,
			timeout:      2 * time.Second,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			namespace := "test-ns"
			ctx := context.Background()

			if tt.setupMock != nil {
				tt.setupMock(client, namespace, tt.resourceName)
			}

			ioCtrl := io.NewIOController()
			var wg sync.WaitGroup
			wg.Add(1)

			err := waitForDeployment(ctx, client, tt.resourceName, tt.condition, namespace, &wg, tt.timeout, ioCtrl)
			wg.Wait()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "Timeout waiting for deployment")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWaitForStatefulset(t *testing.T) {
	tests := []struct {
		name         string
		setupMock    func(kubernetes.Interface, string, string)
		resourceName string
		condition    model.DependsOnCondition
		timeout      time.Duration
		expectError  bool
	}{
		{
			name: "statefulset ready - service_started",
			setupMock: func(c kubernetes.Interface, ns, name string) {
				statefulsets.Deploy(
					context.Background(),
					&appsv1.StatefulSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: ns,
						},
						Status: appsv1.StatefulSetStatus{
							ReadyReplicas: 1,
						},
					},
					c)
			},
			resourceName: "ready-deployment",
			condition:    model.DependsOnServiceRunning,
			timeout:      3 * time.Second,
			expectError:  false,
		},
		{
			name: "statefulset healthy - service_healthy",
			setupMock: func(c kubernetes.Interface, ns, name string) {
				statefulsets.Deploy(
					context.Background(),
					&appsv1.StatefulSet{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: ns,
						},
						Status: appsv1.StatefulSetStatus{
							ReadyReplicas: 1,
						},
					},
					c)
				c.CoreV1().Endpoints(ns).Create(context.Background(), &corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: ns,
					},
					Subsets: []corev1.EndpointSubset{
						{
							Addresses: []corev1.EndpointAddress{},
							Ports:     []corev1.EndpointPort{},
						},
					},
				}, metav1.CreateOptions{})
			},
			resourceName: "healthy-deployment",
			condition:    model.DependsOnServiceHealthy,
			timeout:      3 * time.Second,
			expectError:  false,
		},
		{
			name: "statefulset timeout",
			setupMock: func(c kubernetes.Interface, ns, name string) {
				// intentionally empty to simulate timeout
			},
			resourceName: "missing-deployment",
			condition:    model.DependsOnServiceRunning,
			timeout:      1 * time.Second,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			namespace := "test-ns"
			ctx := context.Background()

			if tt.setupMock != nil {
				tt.setupMock(client, namespace, tt.resourceName)
			}

			ioCtrl := io.NewIOController()
			var wg sync.WaitGroup
			wg.Add(1)

			err := waitForStatefulSet(ctx, client, tt.resourceName, tt.condition, namespace, &wg, tt.timeout, ioCtrl)
			wg.Wait()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "did not reach condition ")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWaitForJob(t *testing.T) {
	tests := []struct {
		name         string
		setupMock    func(kubernetes.Interface, string, string)
		resourceName string
		condition    model.DependsOnCondition
		timeout      time.Duration
		expectError  bool
	}{
		{
			name: "job completed successfully",
			setupMock: func(c kubernetes.Interface, ns, name string) {
				completions := int32(1)
				c.BatchV1().Jobs(ns).Create(context.Background(), &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: ns,
					},
					Spec: batchv1.JobSpec{
						Completions: &completions,
					},
					Status: batchv1.JobStatus{
						Succeeded: 1,
					},
				}, metav1.CreateOptions{})
			},
			resourceName: "successful-job",
			condition:    model.DependsOnServiceCompleted,
			timeout:      3 * time.Second,
			expectError:  false,
		},
		{
			name: "job not completed - timeout",
			setupMock: func(c kubernetes.Interface, ns, name string) {
				completions := int32(1)
				c.BatchV1().Jobs(ns).Create(context.Background(), &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: ns,
					},
					Spec: batchv1.JobSpec{
						Completions: &completions,
					},
					Status: batchv1.JobStatus{
						Succeeded: 0, // Never completes
					},
				}, metav1.CreateOptions{})
			},
			resourceName: "pending-job",
			condition:    model.DependsOnServiceCompleted,
			timeout:      2 * time.Second,
			expectError:  true,
		},
		{
			name:         "unsupported condition",
			setupMock:    func(c kubernetes.Interface, ns, name string) {},
			resourceName: "unsupported-job",
			condition:    model.DependsOnServiceRunning,
			timeout:      2 * time.Second,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			namespace := "test-ns"
			ctx := context.Background()

			if tt.setupMock != nil {
				tt.setupMock(client, namespace, tt.resourceName)
			}

			ioCtrl := io.NewIOController()
			var wg sync.WaitGroup
			wg.Add(1)

			err := waitForJob(ctx, client, tt.resourceName, tt.condition, namespace, &wg, tt.timeout, ioCtrl)
			wg.Wait()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
