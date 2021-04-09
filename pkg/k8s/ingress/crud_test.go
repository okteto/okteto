package ingress

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	extensions "k8s.io/api/extensions/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
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
	ctx := context.Background()
	ingress := &extensions.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake",
			Namespace: "test",
		},
	}

	clientset := fake.NewSimpleClientset(ingress)
	err := Destroy(ctx, ingress.Name, ingress.Namespace, clientset)
	if err != nil {
		t.Fatal(err)
	}
	_, err = clientset.ExtensionsV1beta1().Ingresses(ingress.Namespace).Get(ctx, ingress.Name, metav1.GetOptions{})
	if err == nil {
		t.Fatal(err)
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

	if !reflect.DeepEqual(retrieved, updatedIngress) {
		t.Fatalf("Didn't updated correctly")
	}
}
