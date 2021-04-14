package ingress

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	extensions "k8s.io/api/extensions/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8sTesting "k8s.io/client-go/testing"
)

func TestCreate(t *testing.T) {
	ctx := context.Background()
	ingress := &extensions.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake",
			Namespace: "test",
		},
	}

	clientset := fake.NewSimpleClientset()
	err := Create(ctx, ingress, clientset)
	if err != nil {
		t.Fatal(err)
	}
	retrieved, err := clientset.ExtensionsV1beta1().Ingresses(ingress.Namespace).Get(ctx, ingress.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(retrieved, ingress) {
		t.Fatalf("Didn't created correctly")
	}
}

func TestList(t *testing.T) {
	ctx := context.Background()
	ingress := &extensions.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake",
			Namespace: "test",
		},
	}

	clientset := fake.NewSimpleClientset(ingress)
	ingressList, err := List(ctx, ingress.Namespace, "", clientset)
	if err != nil {
		t.Fatal(err)
	}

	if len(ingressList) != 1 {
		t.Fatal(fmt.Errorf("Expected 1 ingress, found %d", len(ingressList)))
	}

}

func TestDestroy(t *testing.T) {
	var tests = []struct {
		name        string
		ingressName string
		namespace   string
		ingress     *extensions.Ingress
	}{
		{
			name:        "existent-ingress",
			ingressName: "ingress-test",
			namespace:   "test",
			ingress: &extensions.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-test",
					Namespace: "test",
				},
			},
		},
		{
			name:        "ingress-not-found",
			ingressName: "ingress-test",
			namespace:   "test",
			ingress: &extensions.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-existent-ingress",
					Namespace: "another-space",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := fake.NewSimpleClientset(tt.ingress)

			err := Destroy(ctx, tt.ingressName, tt.namespace, client)

			if err != nil {
				t.Fatalf("unexpected error '%s'", err)
			}
		})
	}
}

func TestDestroyWithError(t *testing.T) {
	ctx := context.Background()
	ingressName := "ingress-test"
	namespace := "test"

	kubernetesError := "something went wrong in the test"
	client := fake.NewSimpleClientset()
	client.Fake.PrependReactor("delete", "ingresses", func(action k8sTesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New(kubernetesError)
	})

	err := Destroy(ctx, ingressName, namespace, client)

	if err == nil {
		t.Fatal("an error was expected but no error was returned")
	}
	if !strings.Contains(err.Error(), kubernetesError) {
		t.Fatalf("Got '%s' error but expected '%s'", err.Error(), kubernetesError)
	}
}

func TestUpdate(t *testing.T) {
	ctx := context.Background()
	labels := map[string]string{"key": "value"}
	ingress := &extensions.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake",
			Namespace: "test",
			Labels:    labels,
		},
	}

	clientset := fake.NewSimpleClientset(ingress)

	updatedLabels := map[string]string{"key": "value", "key2": "value2"}
	updatedIngress := &extensions.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake",
			Namespace: "test",
			Labels:    updatedLabels,
		},
	}
	err := Update(ctx, updatedIngress, clientset)
	if err != nil {
		t.Fatal(err)
	}

	retrieved, err := clientset.ExtensionsV1beta1().Ingresses(ingress.Namespace).Get(ctx, ingress.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(retrieved, updatedIngress) {
		t.Fatalf("Didn't updated correctly")
	}
}
