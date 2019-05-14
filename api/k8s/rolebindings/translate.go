package rolebindings

import (
	"github.com/okteto/app/api/k8s/client"
	"github.com/okteto/app/api/model"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func translate(m *model.Member, s *model.Space) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.ID,
			Namespace: s.ID,
		},
		Subjects: []rbacv1.Subject{
			rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      m.ID,
				Namespace: client.GetOktetoNamespace(),
			},
			rbacv1.Subject{
				Kind:      "ServiceAccount",
				Name:      "default",
				Namespace: s.ID,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     s.ID,
		},
	}
}
