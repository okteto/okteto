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
	"github.com/okteto/okteto/pkg/env"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const oktetoLocalRegistryStoreEnabledEnvVarKey = "OKTETO_LOCAL_REGISTRY_STORE_ENABLED"

var oktetoRegistry = ""

func newDockerAndOktetoAuthProvider(registryURL, username, password string, authContext authProviderContextInterface, stderr io.Writer) *authProvider {
	result := &authProvider{
		config:          config.LoadDefaultConfigFile(stderr),
		externalAuth:    authContext.getExternalRegistryCreds,
		newOktetoClient: okteto.NewOktetoClientStateless,
		authContext:     authContext,
	}
	oktetoRegistry = registryURL
	result.config.AuthConfigs[registryURL] = types.AuthConfig{
		ServerAddress: registryURL,
		Username:      username,
		Password:      password,
	}
	return result
}

type authProviderContextInterface interface {
	isOktetoContext() bool
	getOktetoClientCfg() *okteto.ClientCfg
	getExternalRegistryCreds(registryOrImage string, isOkteto bool, c *okteto.Client) (string, string, error)
}

type authProviderContext struct {
	context  string
	token    string
	cert     string
	isOkteto bool
}

func (apc *authProviderContext) isOktetoContext() bool {
	return apc.isOkteto
}

func (apc *authProviderContext) getOktetoClientCfg() *okteto.ClientCfg {
	return &okteto.ClientCfg{
		CtxName: apc.context,
		Token:   apc.token,
		Cert:    apc.cert,
	}
}

func (apc *authProviderContext) getExternalRegistryCreds(registryOrImage string, isOkteto bool, c *okteto.Client) (string, string, error) {
	return okteto.GetExternalRegistryCredentialsStateless(registryOrImage, isOkteto, c)
}

type externalRegistryCredentialFunc func(host string, isOkteto bool, client *okteto.Client) (string, string, error)

type newOktetoClientFunc func(cfg *okteto.ClientCfg, opts ...okteto.Option) (*okteto.Client, error)

type authProvider struct {
	authContext authProviderContextInterface
	config      *configfile.ConfigFile

	// externalAuth is an external registry credentials getter that live
	// outside of the configfile. It is used to load external auth data without
	// going through the target config file store
	externalAuth externalRegistryCredentialFunc

	newOktetoClient newOktetoClientFunc

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

// Credentials returns the credentials for the given host.
// If the host is the okteto registry, it returns the credentials from the config file.
// If the host is not the okteto registry and the OKTETO_LOCAL_REGISTRY_STORE_ENABLED is false or unset, it returns the credentials retrieved from the okteto credentials store.
// If the host is not the okteto registry, it returns the credentials from the config file if the OKTETO_LOCAL_REGISTRY_STORE_ENABLED is true.
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

	ocfg := ap.authContext.getOktetoClientCfg()
	c, err := ap.newOktetoClient(ocfg)
	if err != nil {
		return nil, err
	}

	credentials := ap.getOktetoCredentials(originalHost, c)

	retrieveFromLocal := env.LoadBooleanOrDefault(oktetoLocalRegistryStoreEnabledEnvVarKey, false)
	if !retrieveFromLocal {
		return credentials, nil
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

	if ac.Username != "" && ac.Password != "" {
		credentials = &auth.CredentialsResponse{
			Username: ac.Username,
			Secret:   ac.Password,
		}
	}

	return credentials, nil
}

func (ap *authProvider) getOktetoCredentials(host string, c *okteto.Client) *auth.CredentialsResponse {
	res := &auth.CredentialsResponse{}
	if user, pass, err := ap.externalAuth(host, ap.authContext.isOktetoContext(), c); err != nil {
		oktetoLog.Debugf("failed to load external auth for %s: %w", host, err.Error())
	} else {
		res.Username = user
		res.Secret = pass
	}
	return res
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
