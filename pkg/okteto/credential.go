package okteto

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/okteto/okteto/pkg/k8s/client"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Credentials top body answer
type Credentials struct {
	Credentials Credential
}

//Credential represents an Okteto Space k8s credentials
type Credential struct {
	Server      string `json:"server" yaml:"server"`
	Certificate string `json:"certificate" yaml:"certificate"`
	Token       string `json:"token" yaml:"token"`
	Namespace   string `json:"namespace" yaml:"namespace"`
}

// GetCredentials returns the space config credentials
func GetCredentials(ctx context.Context, namespace string) (*Credential, error) {
	q := fmt.Sprintf(`query{
		credentials(space: "%s"){
			server, certificate, token, namespace
		},
	}`, namespace)

	var cred Credentials
	if err := query(ctx, q, &cred); err != nil {
		return nil, err
	}

	return &cred.Credentials, nil
}

// GetOktetoInternalNamespaceClient returns a k8s client to the okteto internal namepsace
func GetOktetoInternalNamespaceClient(ctx context.Context) (*kubernetes.Clientset, *rest.Config, string, error) {
	cred, err := GetCredentials(ctx, "")
	if err != nil {
		return nil, nil, "", err
	}
	internalNamespace := fmt.Sprintf("%s-okteto", cred.Namespace)

	file, err := ioutil.TempFile("", "okteto")
	if err != nil {
		return nil, nil, "", err
	}
	defer os.Remove(file.Name())

	if err := SetKubeConfig(cred, file.Name(), internalNamespace, GetUserID(), "okteto"); err != nil {
		return nil, nil, "", err
	}
	if err := os.Setenv("KUBECONFIG", file.Name()); err != nil {
		return nil, nil, "", fmt.Errorf("couldn't set the KUBECONFIG environment variable: %w", err)
	}

	c, restConfig, _, err := client.GetLocal()
	if err != nil {
		return nil, nil, "", err
	}
	return c, restConfig, internalNamespace, nil
}
