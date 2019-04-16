package okteto

import (
	"github.com/okteto/app/cli/pkg/errors"
	"github.com/okteto/app/cli/pkg/log"
)

// Credentials top body answer
type Credentials struct {
	Credentials Credential
}

// Credential field answer
type Credential struct {
	Config string
}

// GetK8sB64Config returns the space config credentials
func GetK8sB64Config() (string, error) {
	q := ` query{
			credentials{
				config
			},
		}`

	var cred Credentials
	if err := query(q, &cred); err != nil {
		log.Infof("couldn't get credentials from grapqhl endpoint: %s", err)
		return "", errors.ErrNotLogged
	}

	return cred.Credentials.Config, nil
}
