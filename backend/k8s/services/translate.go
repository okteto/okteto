package services

import (
	"github.com/okteto/app/backend/model"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func translate(dev *model.Dev, s *model.Space) *apiv1.Service {
	var port int32 = 8000
	return &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: dev.Name,
		},
		Spec: apiv1.ServiceSpec{
			Selector: map[string]string{"app": dev.Name},
			Type:     apiv1.ServiceTypeClusterIP,
			Ports: []apiv1.ServicePort{
				apiv1.ServicePort{
					Name:       "p8000",
					Port:       port,
					TargetPort: intstr.IntOrString{StrVal: "8000"},
				},
			},
		},
	}
}
