package ingresses

import (
	"github.com/okteto/app/api/model"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func translate(dev *model.Dev, s *model.Space) *v1beta1.Ingress {
	return &v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: dev.Name,
			Annotations: map[string]string{
				"kubernetes.io/ingress.class": "nginx",
				"kubernetes.io/tls-acme":      "true",
			},
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				v1beta1.IngressRule{
					Host: dev.GetEndpoint(s),
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								v1beta1.HTTPIngressPath{
									Path: "/",
									Backend: v1beta1.IngressBackend{
										ServiceName: dev.Name,
										ServicePort: intstr.FromInt(8000),
									},
								},
							},
						},
					},
				},
			},
			TLS: []v1beta1.IngressTLS{
				v1beta1.IngressTLS{
					SecretName: dev.CertificateName(),
					Hosts:      []string{dev.GetEndpoint(s)},
				},
			},
		},
	}
}
