package ingresses

import (
	"github.com/okteto/okteto/pkg/model"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Translate Endpoint to Ingress
// Translate Service to Ingress

type TranslateOptions struct {
	Name      string
	Namespace string
	IsCompose bool
}

// TranslateEndpoint translates the endpoints spec at compose or okteto manifest and returns an ingress
func TranslateEndpoint(endpointName string, endpoint model.Endpoint, opts *TranslateOptions) *Ingress {
	if endpointName == "" {
		endpointName = opts.Name
	}
	return &Ingress{
		V1:      translateEndpointIngressV1(endpointName, endpoint, opts),
		V1Beta1: translateEndpointIngressV1Beta1(endpointName, endpoint, opts),
	}
}

func translateEndpointIngressV1(ingressName string, endpoint model.Endpoint, opts *TranslateOptions) *networkingv1.Ingress {
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ingressName,
			Namespace:   opts.Namespace,
			Labels:      setLabels(endpoint, opts),
			Annotations: setAnnotations(endpoint),
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: translateEndpointRulesToPathsV1(endpoint),
						},
					},
				},
			},
		},
	}
}

func translateEndpointIngressV1Beta1(ingressName string, endpoint model.Endpoint, opts *TranslateOptions) *networkingv1beta1.Ingress {
	return &networkingv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ingressName,
			Namespace:   opts.Namespace,
			Labels:      setLabels(endpoint, opts),
			Annotations: setAnnotations(endpoint),
		},
		Spec: networkingv1beta1.IngressSpec{
			Rules: []networkingv1beta1.IngressRule{
				{
					IngressRuleValue: networkingv1beta1.IngressRuleValue{
						HTTP: &networkingv1beta1.HTTPIngressRuleValue{
							Paths: translateEndpointRulesToPathsV1Beta1(endpoint),
						},
					},
				},
			},
		},
	}
}

func setLabels(endpoint model.Endpoint, opts *TranslateOptions) map[string]string {
	// init with default label
	labels := model.Labels{
		model.DeployedByLabel: opts.Name,
	}

	if _, ok := labels[model.StackNameLabel]; !ok && opts.IsCompose {
		labels[model.StackNameLabel] = opts.Name
	}

	// append labels from the endpoint spec
	for k := range endpoint.Labels {
		labels[k] = endpoint.Labels[k]
	}
	return labels
}

func setAnnotations(endpoint model.Endpoint) map[string]string {
	// init with defaul annotation
	annotations := model.Annotations{
		model.OktetoIngressAutoGenerateHost: "true",
	}
	for k := range endpoint.Annotations {
		annotations[k] = endpoint.Annotations[k]
	}
	return annotations
}

func translateEndpointRulesToPathsV1(endpoint model.Endpoint) []networkingv1.HTTPIngressPath {
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

func translateEndpointRulesToPathsV1Beta1(endpoint model.Endpoint) []networkingv1beta1.HTTPIngressPath {
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
