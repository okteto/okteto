// Copyright 2023 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
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
