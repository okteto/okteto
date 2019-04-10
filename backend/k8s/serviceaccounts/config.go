package serviceaccounts

import (
	"encoding/base64"
	"fmt"

	"github.com/okteto/app/backend/model"
)

const (
	configTemplate = `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: %s
    server: https://%s
  name: okteto-space
contexts:
- context:
    cluster: okteto-space
    user: okteto-user
  name: okteto-context
current-context: okteto-context
kind: Config
preferences: {}
users:
- name: okteto-user
  user:
    token: %s
`
)

func getConfigB64(s *model.Space, caCert, token string) string {
	endpoint := "35.204.232.187" //TODO: set envvar
	encodedCaCert := base64.StdEncoding.EncodeToString([]byte(caCert))
	configValue := fmt.Sprintf(
		configTemplate,
		encodedCaCert,
		endpoint,
		token,
	)
	encoded := base64.StdEncoding.EncodeToString([]byte(configValue))
	return encoded
}
