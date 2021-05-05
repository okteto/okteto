package diverts

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	k8sScheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type DivertV1nterface interface {
	Diverts(namespace string) DivertInterface
}

type DivertV1Client struct {
	restClient rest.Interface
	scheme     *runtime.Scheme
}

func NewForConfig(cfg *rest.Config) (*DivertV1Client, error) {
	scheme := runtime.NewScheme()
	SchemeBuilder := runtime.NewSchemeBuilder(addKnownTypes)
	if err := SchemeBuilder.AddToScheme(scheme); err != nil {
		return nil, err
	}
	config := *cfg
	config.GroupVersion = &SchemeGroupVersion
	config.APIPath = "/apis"
	config.ContentType = runtime.ContentTypeJSON
	config.NegotiatedSerializer = k8sScheme.Codecs.WithoutConversion()

	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &DivertV1Client{restClient: client, scheme: scheme}, nil

}

func GetClient(thisContext string) (*DivertV1Client, error) {
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{
			CurrentContext: thisContext,
			ClusterInfo:    clientcmdapi.Cluster{Server: ""},
		},
	)
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	c, err := NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize diverts client: %s", err.Error())
	}

	return c, nil
}

func (c *DivertV1Client) Diverts(namespace string) DivertInterface {
	return &divertClient{
		restClient: c.restClient,
		scheme:     c.scheme,
		ns:         namespace,
	}
}
