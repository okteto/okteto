package namespaces

import (
	"github.com/okteto/app/api/model"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func translate(s *model.Space) *v1.Namespace {
	return &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: s.ID,
			Labels: map[string]string{
				"app.kubernetes.io/name": s.ID,
			},
		},
	}
}
