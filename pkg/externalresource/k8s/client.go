package k8s

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	k8sScheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"

	"k8s.io/client-go/rest"
)

const (
	// GroupName k8s group name for the external resource
	GroupName = "dev.okteto.com"
	// GroupVersion k8s version for ExternalResource resource
	GroupVersion = "v1"
)

var (
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

var schemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: GroupVersion}

// ExternalResourceV1Interface defines a method to get a ExternalResourceInterface
type ExternalResourceV1Interface interface {
	ExternalResources(namespace string) ExternalResourceInterface
}

// ExternalResourceClient client to work with ExternalResources v1 resources
type ExternalResourceV1Client struct {
	restClient rest.Interface
	scheme     *runtime.Scheme
}

func GetExternalClient(cfg clientcmdapi.Config) (ExternalResourceV1Interface, error) {
	clientConfig := clientcmd.NewDefaultClientConfig(cfg, nil)

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	c, err := NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// NewForConfig creates a new client for ExternalResourceV1 or error
func NewForConfig(cfg *rest.Config) (*ExternalResourceV1Client, error) {
	scheme := runtime.NewScheme()
	if err := SchemeBuilder.AddToScheme(scheme); err != nil {
		return nil, err
	}
	config := *cfg
	config.GroupVersion = &schemeGroupVersion
	config.APIPath = "/apis"
	config.ContentType = runtime.ContentTypeJSON
	config.NegotiatedSerializer = k8sScheme.Codecs.WithoutConversion()

	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &ExternalResourceV1Client{restClient: client, scheme: scheme}, nil

}

// ExternalResources returns an instance of ExternalResourceInterface
func (c *ExternalResourceV1Client) ExternalResources(namespace string) ExternalResourceInterface {
	return &externalClient{
		restClient: c.restClient,
		scheme:     c.scheme,
		ns:         namespace,
	}
}

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(schemeGroupVersion,
		&External{},
		&ExternalList{},
	)

	metav1.AddToGroupVersion(scheme, schemeGroupVersion)
	return nil
}
