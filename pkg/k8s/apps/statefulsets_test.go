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

func TestSfsGetDevCloneWithError(t *testing.T) {
	sfs := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
	}
	app := StatefulSetApp{kind: okteto.StatefulSet, sfs: sfs}
	c := fake.NewSimpleClientset()
	ctx := context.Background()

	_, err := app.GetDevClone(ctx, c)

	require.Error(t, err)
}

func TestSfsGetDevCloneWithoutError(t *testing.T) {
	sfs := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
	}

	cloned := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-okteto",
			Namespace: "test",
			Labels: map[string]string{
				"dev.okteto.com/clone": "true",
			},
		},
	}

	app := StatefulSetApp{kind: okteto.StatefulSet, sfs: sfs}
	c := fake.NewSimpleClientset(cloned)
	ctx := context.Background()
	expected := &StatefulSetApp{kind: okteto.StatefulSet, sfs: cloned}

	result, err := app.GetDevClone(ctx, c)

	require.NoError(t, err)
	require.Equal(t, expected, result)
}
