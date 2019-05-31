package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/okteto/webhooks/pkg/admission"
	"github.com/okteto/webhooks/pkg/ingress"
	"github.com/okteto/webhooks/pkg/service"
	adminv1beta1 "k8s.io/api/admission/v1beta1"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	tlsDir      = `/run/secrets/tls`
	tlsCertFile = `tls.crt`
	tlsKeyFile  = `tls.key`
)

var (
	clientset       *kubernetes.Clientset
	okResponse = []byte(`{'status': 'ok'}`)
	ingressResource = metav1.GroupVersionResource{Version: "v1beta1", Resource: "ingresses", Group: "extensions"}
	serviceResource = metav1.GroupVersionResource{Version: "v1", Resource: "services", Group: ""}
)

func applyIngressDefaults(req *adminv1beta1.AdmissionRequest) ([]admission.PatchOperation, error) {
	if req.Resource != ingressResource {
		log.Printf("expect resource to be %s, but got %s", ingressResource, req.Resource)
		return nil, nil
	}

	i := &v1beta1.Ingress{}
	if err := json.Unmarshal(req.Object.Raw, i); err != nil {
		return nil, fmt.Errorf("could not deserialize ingress object: %v", err)
	}

	if err := ingress.Validate(clientset, i); err != nil {
		return nil, err
	}

	return []admission.PatchOperation{}, nil
}

func applyServiceDefaults(req *adminv1beta1.AdmissionRequest) ([]admission.PatchOperation, error) {
	if req.Resource != serviceResource {
		log.Printf("expect resource to be %s, but got %s", serviceResource, req.Resource)
		return nil, nil
	}

	if req.Operation == adminv1beta1.Delete {
		if err := service.DeleteIngressIfDefault(clientset, req.Namespace, req.Name); err != nil {
			log.Printf(err.Error())
		}

		return nil, nil
	}

	s := &apiv1.Service{}
	if err := json.Unmarshal(req.Object.Raw, s); err != nil {
		log.Printf("could not deserialize service object: %v \n %s", err, string(req.Object.Raw))
		return nil, nil
	}

	if err := service.CreateIngressIfDefault(clientset, s); err != nil {
		log.Printf(err.Error())
	}

	return nil, nil
}

func main() {
	certPath := filepath.Join(tlsDir, tlsCertFile)
	keyPath := filepath.Join(tlsDir, tlsKeyFile)

	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err.Error())
	}

	clientset, err = kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		log.Fatal(err.Error())
	}

	mux := http.NewServeMux()
	mux.Handle("/mutate/ingress", admission.AdmitFuncHandler(applyIngressDefaults))
	mux.Handle("/mutate/service", admission.AdmitFuncHandler(applyServiceDefaults))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {	
		w.Write(okResponse)
		return
	})

	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "8443"
	}

	addr := fmt.Sprintf(":%s", port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.Printf("listening for requests on %s", addr)
	log.Fatal(server.ListenAndServeTLS(certPath, keyPath))
}
