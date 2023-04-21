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

import authenticationv1 "k8s.io/api/authentication/v1"

// NoopCache is a cache that does nothing
type NoopCache struct{}

// Set does nothing
func (n NoopCache) Set(contextName, namespace string, token authenticationv1.TokenRequest) {
	// do nothing
}

// Get does nothing
func (n NoopCache) Get(contextName, namespace string) (string, error) {
	return "", nil
}
