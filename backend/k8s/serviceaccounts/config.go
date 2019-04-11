package serviceaccounts

import (
	"encoding/base64"
	"fmt"
	"os"

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
	endpoint := os.Getenv("CLUSTER_PUBLIC_ENDPOINT")
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
