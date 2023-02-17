package apps

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDeploymentGetDevCloneWithError(t *testing.T) {
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
	}
	app := DeploymentApp{kind: okteto.Deployment, d: d}
	c := fake.NewSimpleClientset()
	ctx := context.Background()

	_, err := app.GetDevClone(ctx, c)

	require.Error(t, err)
}

func TestDeploymentGetDevCloneWithoutError(t *testing.T) {
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
	}

	cloned := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-okteto",
			Namespace: "test",
			Labels: map[string]string{
				"dev.okteto.com/clone": "true",
			},
		},
	}

	app := DeploymentApp{kind: okteto.Deployment, d: d}
	c := fake.NewSimpleClientset(cloned)
	ctx := context.Background()
	expected := &DeploymentApp{kind: okteto.Deployment, d: cloned}

	result, err := app.GetDevClone(ctx, c)

	require.NoError(t, err)
	require.Equal(t, expected, result)
}
