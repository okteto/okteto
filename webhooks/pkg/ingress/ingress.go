package ingress

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	baseHost        = ""
	ingressResource = metav1.GroupVersionResource{Version: "v1beta1", Resource: "ingresses", Group: "extensions"}
)

const (
	basenameLabel         = "dev.okteto.com/basename"
	certManagerAnnotation = "certmanager.k8s.io"
	defaultBaseHost       = "cloud.okteto.net"
)

func init() {
	baseHost = os.Getenv("OKTETO_BASE_DOMAIN")
	if len(baseHost) == 0 {
		baseHost = defaultBaseHost
	}
}

// BuildAllowedURLSuffix returns the allowed prefix
func BuildAllowedURLSuffix(clientset *kubernetes.Clientset, namespace, name string) string {
	n, err := clientset.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	if err != nil {
		log.Printf("failed to get namespace  for ingress %s/%s: %s", namespace, name, err)
		return baseHost
	}

	if l, ok := n.GetObjectMeta().GetLabels()[basenameLabel]; ok {
		return fmt.Sprintf("-%s.%s", l, baseHost)
	}

	log.Printf("namespace %s didn't have label %s", namespace, basenameLabel)
	return baseHost
}

// Validate returns an error if the ingress has invalid hosts or tags
func Validate(clientset *kubernetes.Clientset, ingress *v1beta1.Ingress) error {
	annotations := ingress.GetAnnotations()
	for k := range annotations {
		if strings.Contains(k, certManagerAnnotation) {
			log.Printf("ingress on namespace %s with annotation %s rejected", ingress.GetNamespace(), k)
			return errors.New("Interested in cert-manager support? Contact us at sales@okteto.com")
		}
	}

	suffix := BuildAllowedURLSuffix(clientset, ingress.GetNamespace(), ingress.GetName())

	for _, r := range ingress.Spec.Rules {
		if !strings.HasSuffix(r.Host, suffix) {
			log.Printf("ingress on namespace %s with host %s rejected", ingress.GetNamespace(), r.Host)
			return fmt.Errorf("Host %s must match the `*%s` pattern. Interested in custom hosts support? Contact us at sales@okteto.com", r.Host, suffix)
		}
	}

	for _, t := range ingress.Spec.TLS {
		for _, h := range t.Hosts {
			if !strings.HasSuffix(h, suffix) {
				log.Printf("ingress on namespace %s with TLS host %s rejected", ingress.GetNamespace(), h)
				return fmt.Errorf("TLS host %s must match the `*%s` pattern. Interested in custom TLS hosts support? Contact us at sales@okteto.com", h, suffix)
			}
		}
	}

	return nil
}
