package services

import (
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	oktetoAutoCreateAnnotation = "dev.okteto.com/auto-ingress"
)

func translate(dev *model.Dev) *apiv1.Service {
	return &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dev.Name,
			Namespace: dev.Namespace,
			Annotations: map[string]string{
				oktetoAutoCreateAnnotation: "true",
			},
		},
		Spec: apiv1.ServiceSpec{
			Selector: map[string]string{"app": dev.Name},
			Type:     apiv1.ServiceTypeClusterIP,
			Ports: []apiv1.ServicePort{
				apiv1.ServicePort{
					Name:       dev.Name,
					Port:       8080,
					TargetPort: intstr.IntOrString{StrVal: "8080"},
				},
			},
		},
	}
}
