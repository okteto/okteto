// Copyright 2021 The Okteto Authors
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

package registry

import (
	"net/http"
	"strings"

	"github.com/heroku/docker-registry-client/registry"
	"github.com/okteto/okteto/pkg/log"
)

// NewRegistryClient creates a new Registry with the given URL and credentials, then Ping()s it
// before returning it to verify that the registry is available.
func NewRegistryClient(registryURL, username, password string) (*registry.Registry, error) {
	transport := http.DefaultTransport
	return newFromTransport(registryURL, username, password, transport)
}

func newFromTransport(registryURL, username, password string, transport http.RoundTripper) (*registry.Registry, error) {
	url := strings.TrimSuffix(registryURL, "/")
	transport = registry.WrapTransport(transport, url, username, password)
	registry := &registry.Registry{
		URL: url,
		Client: &http.Client{
			Transport: transport,
		},
		Logf: log.Infof,
	}

	return registry, nil
}
