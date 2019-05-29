package services

import (
	"github.com/okteto/app/api/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

//Translate returns a k8s service for a dev environment
func Translate(dev *model.Dev, s *model.Space) *apiv1.Service {
	return &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: dev.Name,
		},
		Spec: apiv1.ServiceSpec{
			Selector: map[string]string{"app": dev.Name},
			Type:     apiv1.ServiceTypeClusterIP,
			Ports: []apiv1.ServicePort{
				apiv1.ServicePort{
					Name:       "p8080",
					Port:       8080,
					TargetPort: intstr.IntOrString{StrVal: "8080"},
				},
			},
		},
	}
}
