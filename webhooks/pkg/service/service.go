package service

import (
	"fmt"
	"log"

	"github.com/okteto/webhooks/pkg/ingress"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

const (
	autoIngressLabel = "dev.okteto.com/auto-ingress"
)

var (
	serviceResource = metav1.GroupVersionResource{Version: "v1", Resource: "services", Group: ""}
)

// DeleteIngressIfDefault deletes the default ingress that matches the service deleted
func DeleteIngressIfDefault(clientset *kubernetes.Clientset, namespace, name string) error {
	iName := fmt.Sprintf("okteto-%s", name)

	i, err := clientset.ExtensionsV1beta1().Ingresses(namespace).Get(iName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("default ingress doesn't exist for %s/%s: %s", namespace, name, err)
	}

	if !autoIngressEnabled(i.GetAnnotations()) {
		return fmt.Errorf("ingress %s/%s wasn't created by okteto: %+v", i.Namespace, i.Name, i.GetAnnotations())
	}

	err = clientset.ExtensionsV1beta1().Ingresses(namespace).Delete(iName, &metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete default ingress: %s", err)
	}

	log.Printf("deleted default ingress %s for %s/%s", i.Name, namespace, name)
	return nil
}

// CreateIngressIfDefault creates a default ingress if the service has the dev.okteto.com/auto-ingress annotation
func CreateIngressIfDefault(clientset *kubernetes.Clientset, service *apiv1.Service) error {
	if !autoIngressEnabled(service.GetAnnotations()) {
		log.Printf("service %s/%s doesn't require an auto-ingress", service.GetNamespace(), service.GetName())
		return nil
	}

	if len(service.Spec.Ports) == 0 {
		return fmt.Errorf("service %s/%s doesn't have published ports", service.Namespace, service.Name)
	}

	host := fmt.Sprintf("%s%s", service.GetName(), ingress.BuildAllowedURLSuffix(clientset, service.GetNamespace(), service.GetName()))
	iName := fmt.Sprintf("okteto-%s", service.GetName())
	i := getDefaultIngress(iName, host, service.GetName(), int(service.Spec.Ports[0].Port))
	_, err := clientset.ExtensionsV1beta1().Ingresses(service.GetNamespace()).Create(i)
	if err != nil {
		return fmt.Errorf("failed to create default ingress for service %s/%s: %s", service.Namespace, service.Name, err)
	}

	log.Printf("created default ingress for %s/%s", service.GetNamespace(), service.GetName())
	return nil
}

func autoIngressEnabled(a map[string]string) bool {
	k, ok := a[autoIngressLabel]
	if !ok || k != "true" {
		return false
	}

	return true
}

func getDefaultIngress(name, host, service string, port int) *v1beta1.Ingress {
	return &v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				"kubernetes.io/ingress.class": "nginx",
				autoIngressLabel:              "true",
			},
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				v1beta1.IngressRule{
					Host: host,
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								v1beta1.HTTPIngressPath{
									Path: "/",
									Backend: v1beta1.IngressBackend{
										ServiceName: service,
										ServicePort: intstr.FromInt(port),
									},
								},
							},
						},
					},
				},
			},
			TLS: []v1beta1.IngressTLS{
				v1beta1.IngressTLS{
					Hosts: []string{host},
				},
			},
		},
	}
}
