package postgres

import (
	"github.com/okteto/app/api/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

//TranslateService returns the service for postgress
func TranslateService(s *model.Space) *apiv1.Service {
	return &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      model.POSTGRES,
			Namespace: s.ID,
		},
		Spec: apiv1.ServiceSpec{
			Selector: map[string]string{"app": model.POSTGRES},
			Type:     apiv1.ServiceTypeClusterIP,
			Ports: []apiv1.ServicePort{
				apiv1.ServicePort{
					Name:       "p5432",
					Port:       5432,
					TargetPort: intstr.IntOrString{StrVal: "5432"},
				},
			},
		},
	}
}
