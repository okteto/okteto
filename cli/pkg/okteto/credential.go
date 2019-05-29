package okteto

import (
	"fmt"

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
func GetCredentials(space string) (*Credential, error) {
	q := ""
	if space == "" {
		q = `query{
			credentials{
				server, certificate, token, namespace
			},
		}`
	} else {
		q = fmt.Sprintf(`query {
			credentials(space: "%s") {
				server, certificate, token, namespace
				}
			}`, space)
	}

	var cred Credentials
	if err := query(q, &cred); err != nil {
		log.Infof("couldn't get credentials from grapqhl endpoint: %s", err)
		return nil, errors.ErrNotLogged
	}

	return &cred.Credentials, nil
}
