package model

import (
	"github.com/okteto/okteto/pkg/k8s/ingresses"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	"k8s.io/apimachinery/pkg/util/intstr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TranslateEndpointsToIngress(manifest *Manifest) []*ingresses.Ingress {
	endpoints := manifest.Deploy.Endpoints

	var igs = make([]*ingresses.Ingress, 0)
	for ingressName, endpointSpec := range endpoints {
		meta := metav1.ObjectMeta{
			Name:        ingressName,
			Namespace:   manifest.Namespace,
			Labels:      addIngressDefaultLabels(manifest, endpointSpec),
			Annotations: addIngressDefaultAnnotations(endpointSpec),
		}

		ig := &ingresses.Ingress{
			V1: &networkingv1.Ingress{
				ObjectMeta: meta,
				Spec:       translateEndpointIngressSpecV1(endpointSpec),
			},
			V1Beta1: &networkingv1beta1.Ingress{
				ObjectMeta: meta,
				Spec:       translateEndpointIngressSpecV1Beta1(endpointSpec),
			},
		}
		igs = append(igs, ig)
	}
	return igs
}

func translateEndpointIngressSpecV1(endpointSpec Endpoint) networkingv1.IngressSpec {
	return networkingv1.IngressSpec{
		Rules: []networkingv1.IngressRule{
			{
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: translateEndpointRulesToPathsV1(endpointSpec),
					},
				},
			},
		},
	}
}

func translateEndpointIngressSpecV1Beta1(endpoint Endpoint) networkingv1beta1.IngressSpec {
	return networkingv1beta1.IngressSpec{
		Rules: []networkingv1beta1.IngressRule{
			{
				IngressRuleValue: networkingv1beta1.IngressRuleValue{
					HTTP: &networkingv1beta1.HTTPIngressRuleValue{
						Paths: translateEndpointRulesToPathsV1Beta1(endpoint),
					},
				},
			},
		},
	}
}

func translateEndpointRulesToPathsV1(endpoint Endpoint) []networkingv1.HTTPIngressPath {
	paths := make([]networkingv1.HTTPIngressPath, 0)
	pathType := networkingv1.PathTypeImplementationSpecific
	for _, rule := range endpoint.Rules {
		path := networkingv1.HTTPIngressPath{
			Path:     rule.Path,
			PathType: &pathType,
			Backend: networkingv1.IngressBackend{
				Service: &networkingv1.IngressServiceBackend{
					Name: rule.Service,
					Port: networkingv1.ServiceBackendPort{
						Number: rule.Port,
					},
				},
			},
		}
		paths = append(paths, path)
	}
	return paths
}

func translateEndpointRulesToPathsV1Beta1(endpoint Endpoint) []networkingv1beta1.HTTPIngressPath {
	paths := make([]networkingv1beta1.HTTPIngressPath, 0)
	for _, rule := range endpoint.Rules {
		path := networkingv1beta1.HTTPIngressPath{
			Path: rule.Path,
			Backend: networkingv1beta1.IngressBackend{
				ServiceName: rule.Service,
				ServicePort: intstr.IntOrString{IntVal: rule.Port},
			},
		}
		paths = append(paths, path)
	}
	return paths
}

func addIngressDefaultAnnotations(endpoint Endpoint) map[string]string {
	annotations := Annotations{
		OktetoIngressAutoGenerateHost: "true",
	}
	for k := range endpoint.Annotations {
		annotations[k] = endpoint.Annotations[k]
	}
	return annotations
}

func addIngressDefaultLabels(manifest *Manifest, endpoint Endpoint) map[string]string {
	labels := Labels{
		DeployedByLabel: manifest.Name,
	}
	if manifest.Deploy.ComposeSection != nil {
		labels[StackNameLabel] = manifest.Name
	}
	for k := range endpoint.Labels {
		labels[k] = endpoint.Labels[k]
	}
	return labels
}
