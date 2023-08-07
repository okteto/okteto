package types

import (
	"encoding/json"
	authenticationv1 "k8s.io/api/authentication/v1"
)

type KubeTokenResponse struct {
	authenticationv1.TokenRequest
}

func (k *KubeTokenResponse) ToJson() (string, error) {
	bytes, err := json.MarshalIndent(k, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
