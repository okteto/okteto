package client

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"

	"bitbucket.org/okteto/okteto/backend/model"
)

const (
	configTemplate = `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: %s
    server: https://%s
  name: okteto-cluster
contexts:
- context:
    cluster: okteto-cluster
    user: okteto-user
  name: okteto-context
current-context: okteto-context
kind: Config
preferences: {}
users:
- name: okteto-user
  user:
    username: %s
    password: %s
`
)

func getConfig(p *model.Provider) (string, error) {
	configFile, err := ioutil.TempFile("", "k8-config")
	if err != nil {
		return "", fmt.Errorf("Error creating tmp file: %s", err)
	}
	configValue := fmt.Sprintf(
		configTemplate,
		base64.StdEncoding.EncodeToString([]byte(p.CaCert)),
		p.Endpoint,
		p.Username,
		p.Password,
	)
	if err := ioutil.WriteFile(configFile.Name(), []byte(configValue), 0400); err != nil {
		return "", err
	}
	return configFile.Name(), nil
}
