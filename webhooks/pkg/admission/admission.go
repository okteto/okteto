package admission

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"k8s.io/api/admission/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

const (
	jsonContentType = `application/json`
	namespaceFile   = `/var/run/secrets/kubernetes.io/serviceaccount/namespace`
)

var (
	universalDeserializer = serializer.NewCodecFactory(runtime.NewScheme()).UniversalDeserializer()
	oktetoNamespace       = "okteto"
)

// PatchOperation has the operation to patch
type PatchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

type admitFunc func(*v1beta1.AdmissionRequest) ([]PatchOperation, error)

func init() {
	b, err := ioutil.ReadFile(namespaceFile)
	if err != nil {
		log.Fatalf("error getting namespace: %s", err)
	}

	oktetoNamespace = string(b)

}
func isKubeNamespace(ns string) bool {
	return ns == metav1.NamespacePublic || ns == metav1.NamespaceSystem || ns == oktetoNamespace
}

func doServeAdmitFunc(w http.ResponseWriter, r *http.Request, admit admitFunc) ([]byte, error) {
	// Step 1: Request validation. Only handle POST requests with a body and json content type.

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil, fmt.Errorf("invalid method %s, only POST requests are allowed", r.Method)
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil, fmt.Errorf("could not read request body: %v", err)
	}

	if contentType := r.Header.Get("Content-Type"); contentType != jsonContentType {
		w.WriteHeader(http.StatusBadRequest)
		return nil, fmt.Errorf("unsupported content type %s, only %s is supported", contentType, jsonContentType)
	}

	// Step 2: Parse the AdmissionReview request.

	var admissionReviewReq v1beta1.AdmissionReview

	if _, _, err := universalDeserializer.Decode(body, nil, &admissionReviewReq); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil, fmt.Errorf("could not deserialize request: %v", err)
	} else if admissionReviewReq.Request == nil {
		w.WriteHeader(http.StatusBadRequest)
		return nil, errors.New("malformed admission review: request is nil")
	}

	// Step 3: Construct the AdmissionReview response.

	admissionReviewResponse := v1beta1.AdmissionReview{
		Response: &v1beta1.AdmissionResponse{
			UID: admissionReviewReq.Request.UID,
		},
	}

	var patchOps []PatchOperation
	// Apply the admit() function only for non-Kubernetes namespaces. For objects in Kubernetes namespaces, return
	// an empty set of patch operations.
	if !isKubeNamespace(admissionReviewReq.Request.Namespace) {
		patchOps, err = admit(admissionReviewReq.Request)
	}

	if err != nil {
		// If the handler returned an error, incorporate the error message into the response and deny the object
		// creation.
		admissionReviewResponse.Response.Allowed = false
		admissionReviewResponse.Response.Result = &metav1.Status{
			Message: err.Error(),
		}
	} else {
		// Otherwise, encode the patch operations to JSON and return a positive response.
		patchBytes, err := json.Marshal(patchOps)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return nil, fmt.Errorf("could not marshal JSON patch: %v", err)
		}
		admissionReviewResponse.Response.Allowed = true
		admissionReviewResponse.Response.Patch = patchBytes
	}

	// Return the AdmissionReview with a response as JSON.
	bytes, err := json.Marshal(&admissionReviewResponse)
	if err != nil {
		return nil, fmt.Errorf("marshaling response: %v", err)
	}

	return bytes, nil
}
func serveAdmitFunc(w http.ResponseWriter, r *http.Request, admit admitFunc) {
	var writeErr error
	if bytes, err := doServeAdmitFunc(w, r, admit); err != nil {
		log.Printf("Error handling webhook request: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, writeErr = w.Write([]byte(err.Error()))
	} else {
		_, writeErr = w.Write(bytes)
	}

	if writeErr != nil {
		log.Printf("Could not write response: %v", writeErr)
	}
}

// AdmitFuncHandler takes an admitFunc and wraps it into a http.Handler by means of calling serveAdmitFunc.
func AdmitFuncHandler(admit admitFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveAdmitFunc(w, r, admit)
	})
}
