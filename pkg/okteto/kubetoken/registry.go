package kubetoken

import (
	"encoding/json"

	authenticationv1 "k8s.io/api/authentication/v1"
)

type storeRegister struct {
	Token authenticationv1.TokenRequest `json:"token"`
}

type key struct {
	ContextName string `json:"context"`
	Namespace   string `json:"namespace"`
}

// storeRegistry is a map of key to storeRegister
type storeRegistry map[key]storeRegister

// These implementations are needed to make the key type marshalable
// With this we are able to use the key type as a map key in the storeRegistry
func (k key) MarshalText() (text []byte, err error) {
	type alias key
	return json.Marshal(alias(k))
}

func (k *key) UnmarshalText(data []byte) error {
	type alias key
	var receiver alias
	if err := json.Unmarshal(data, &receiver); err != nil {
		return err
	}
	*k = key(receiver)
	return nil
}
