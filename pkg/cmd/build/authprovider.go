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

package build

import (
	"context"
	"io"
	"strings"
	"sync"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/types"
	"github.com/moby/buildkit/session/auth"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/okteto/okteto/pkg/okteto"
)

var oktetoRegistry = ""

func newDockerAndOktetoAuthProvider(registryURL, username, password string, stderr io.Writer) *authProvider {
	result := &authProvider{
		config:       config.LoadDefaultConfigFile(stderr),
		externalAuth: okteto.GetExternalRegistryCredentialsWithContext,
	}
	oktetoRegistry = registryURL
	result.config.AuthConfigs[registryURL] = types.AuthConfig{
		ServerAddress: registryURL,
		Username:      username,
		Password:      password,
	}
	return result
}

type externalRegistryCredentialFunc func(ctx context.Context, host string) (string, string, error)

type authProvider struct {
	config *configfile.ConfigFile

	// externalAuth is an external registry credentials getter that live
	// outside of the configfile. It is used to load external auth data without
	// going through the target config file store
	externalAuth externalRegistryCredentialFunc

	// The need for this mutex is not well understood.
	// Without it, the docker cli on OS X hangs when
	// reading credentials from docker-credential-osxkeychain.
	// See issue https://github.com/docker/cli/issues/1862
	mu sync.Mutex
}

func (ap *authProvider) Register(server *grpc.Server) {
	auth.RegisterAuthServer(server, ap)
}

func (*authProvider) FetchToken(_ context.Context, _ *auth.FetchTokenRequest) (*auth.FetchTokenResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method FetchToken not implemented")
}

func (*authProvider) GetTokenAuthority(_ context.Context, _ *auth.GetTokenAuthorityRequest) (*auth.GetTokenAuthorityResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTokenAuthority not implemented")
}

func (*authProvider) VerifyTokenAuthority(_ context.Context, _ *auth.VerifyTokenAuthorityRequest) (*auth.VerifyTokenAuthorityResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method VerifyTokenAuthority not implemented")
}

func (ap *authProvider) Credentials(ctx context.Context, req *auth.CredentialsRequest) (*auth.CredentialsResponse, error) {
	if req.Host == oktetoRegistry {
		return &auth.CredentialsResponse{
			Username: ap.config.AuthConfigs[oktetoRegistry].Username,
			Secret:   ap.config.AuthConfigs[oktetoRegistry].Password,
		}, nil
	}

	ap.mu.Lock()
	defer ap.mu.Unlock()

	originalHost := req.Host
	if req.Host == "registry-1.docker.io" {
		req.Host = "https://index.docker.io/v1/"
	}

	if ap.config.CredentialsStore == "okteto" {
		ap.config.CredentialsStore = ""
	}

	ac, err := ap.config.GetAuthConfig(req.Host)
	if err != nil {
		if isErrCredentialsHelperNotAccessible(err) {
			oktetoLog.Infof("could not access %s defined in %s", ap.config.CredentialsStore, ap.config.Filename)
			return &auth.CredentialsResponse{}, nil
		}

		return nil, err
	}
	if ac.IdentityToken != "" {
		return &auth.CredentialsResponse{
			Secret: ac.IdentityToken,
		}, nil
	}

	res := &auth.CredentialsResponse{
		Username: ac.Username,
		Secret:   ac.Password,
	}

	// local credentials takes precedence over cluster defined credentials
	if res.Username == "" || res.Secret == "" {
		if user, pass, err := ap.externalAuth(ctx, originalHost); err != nil {
			oktetoLog.Debugf("failed to load external auth for %s: %w", req.Host, err.Error())
		} else {
			res.Username = user
			res.Secret = pass
		}
	}

	return res, nil
}

func isErrCredentialsHelperNotAccessible(err error) bool {

	if !strings.HasPrefix(err.Error(), "error getting credentials") {
		return false
	}

	if strings.Contains(err.Error(), "resolves to executable in current directory") {
		return true
	}

	if strings.Contains(err.Error(), "executable file not found") {
		return true
	}

	return false
}
