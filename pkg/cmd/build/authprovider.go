// Copyright 2020 The Okteto Authors
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

package build

import (
	"context"
	"io"
	"sync"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/types"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth"
	"google.golang.org/grpc"
)

var oktetoRegistry = ""

func newDockerAndOktetoAuthProvider(registryURL, username, password string, stderr io.Writer) session.Attachable {
	result := &authProvider{
		config: config.LoadDefaultConfigFile(stderr),
	}
	oktetoRegistry = registryURL
	result.config.AuthConfigs[registryURL] = types.AuthConfig{
		ServerAddress: registryURL,
		Username:      username,
		Password:      password,
	}
	return result
}

type authProvider struct {
	config *configfile.ConfigFile

	// The need for this mutex is not well understood.
	// Without it, the docker cli on OS X hangs when
	// reading credentials from docker-credential-osxkeychain.
	// See issue https://github.com/docker/cli/issues/1862
	mu sync.Mutex
}

func (ap *authProvider) Register(server *grpc.Server) {
	auth.RegisterAuthServer(server, ap)
}

func (ap *authProvider) Credentials(ctx context.Context, req *auth.CredentialsRequest) (*auth.CredentialsResponse, error) {
	res := &auth.CredentialsResponse{}
	if req.Host == oktetoRegistry {
		res.Username = ap.config.AuthConfigs[oktetoRegistry].Username
		res.Secret = ap.config.AuthConfigs[oktetoRegistry].Password
		return res, nil
	}

	ap.mu.Lock()
	defer ap.mu.Unlock()
	if req.Host == "registry-1.docker.io" {
		req.Host = "https://index.docker.io/v1/"
	}
	ac, err := ap.config.GetAuthConfig(req.Host)
	if err != nil {
		return nil, err
	}
	if ac.IdentityToken != "" {
		res.Secret = ac.IdentityToken
	} else {
		res.Username = ac.Username
		res.Secret = ac.Password
	}
	return res, nil
}
