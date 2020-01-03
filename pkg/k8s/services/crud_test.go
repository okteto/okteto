package services

import (
	"testing"

	"github.com/okteto/okteto/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGet(t *testing.T) {
	svc := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake",
			Namespace: "test",
		},
	}

	clientset := fake.NewSimpleClientset(svc)
	s, err := Get(svc.GetNamespace(), svc.GetName(), clientset)
	if err != nil {
		t.Fatal(err)
	}

	if s == nil {
		t.Fatal("empty service")
	}

	if s.Name != svc.GetName() {
		t.Fatalf("wrong service. Got %s, expected %s", s.Name, svc.GetName())
	}

	_, err = Get("missing", "test", clientset)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.IsNotFound(err) {
		t.Fatalf("expected not found error got: %s", err)
	}
}
