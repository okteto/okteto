package okteto

import (
	"github.com/okteto/app/cli/pkg/errors"
	"github.com/okteto/app/cli/pkg/log"
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
func GetCredentials(namespace string) (*Credential, error) {
	q := `query{
		credentials{
			server, certificate, token, namespace
		},
	}`

	var cred Credentials
	if err := query(q, &cred); err != nil {
		log.Infof("couldn't get credentials from grapqhl endpoint: %s", err)
		return nil, errors.ErrNotLogged
	}

	if namespace != "" {
		cred.Credentials.Namespace = namespace
	}

	return &cred.Credentials, nil
}
