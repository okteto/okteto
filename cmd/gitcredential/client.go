// Copyright 2026 The Okteto Authors
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

package gitcredential

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
)

var (
	errURLNotSet   = fmt.Errorf("neither %s nor %s is set", model.OktetoURLEnvVar, model.OktetoContextEnvVar)
	errTokenNotSet = fmt.Errorf("%s is not set", model.OktetoTokenEnvVar)
)

// apiConfig is the Okteto instance and token the helper calls the API with
type apiConfig struct {
	url   string
	token string
	// serverName is the internal address of the Okteto API in remote runs
	serverName string
}

// apiConfigFromEnv reads the Okteto instance from OKTETO_URL or OKTETO_CONTEXT, in
// the same order as the rest of the CLI, and the token from OKTETO_TOKEN. In remote
// runs it also reads the internal address of the API that remote-run gets as --server-name:
// Git starts the helper as a separate process that doesn't get that flag.
func apiConfigFromEnv() (apiConfig, error) {
	oktetoURL := os.Getenv(model.OktetoURLEnvVar)
	if oktetoURL == "" {
		oktetoURL = os.Getenv(model.OktetoContextEnvVar)
	}
	if oktetoURL == "" {
		return apiConfig{}, errURLNotSet
	}
	token := os.Getenv(model.OktetoTokenEnvVar)
	if token == "" {
		return apiConfig{}, errTokenNotSet
	}
	return apiConfig{
		url:        oktetoURL,
		token:      token,
		serverName: os.Getenv(constants.OktetoInternalServerNameEnvVar),
	}, nil
}

// newGetterFromEnv returns the CLI's Okteto API client for the instance and token in
// the env, so proxies, the server name and the context certificate work as in the rest
// of the CLI. It sends the token as is and doesn't load the Okteto context: that calls
// the API and writes the context store on every Git request, and swaps an installer's
// token for the token of its user.
func newGetterFromEnv() (credentialsGetter, error) {
	cfg, err := apiConfigFromEnv()
	if err != nil {
		return nil, err
	}
	okteto.SetServerNameOverride(cfg.serverName)
	client, err := okteto.NewOktetoClientStateless(&okteto.ClientCfg{
		CtxName: cfg.url,
		Token:   cfg.token,
		Cert:    storedCertificate(cfg.url),
	})
	if err != nil {
		return nil, err
	}
	return client.User(), nil
}

// storedCertificate returns the certificate of the Okteto context saved for the
// instance, if there is one. The certificate is optional, so a store that can't be
// read means no certificate. It reads the store itself because okteto.GetContextStore
// exits when it can't.
func storedCertificate(oktetoURL string) string {
	content, err := os.ReadFile(config.GetOktetoContextsStorePath())
	if err != nil {
		return ""
	}
	store, err := decodeContextStore(content)
	if err != nil {
		oktetoLog.Infof("ignoring the Okteto context store: %s", err)
		return ""
	}
	return certificateFor(store, oktetoURL)
}

// decodeContextStore decodes the store as strictly as okteto.GetContextStore does. The
// client checks a certificate that the system doesn't trust against the current
// context, which loads the store again and exits on a store this would accept.
func decodeContextStore(content []byte) (*okteto.ContextStore, error) {
	dec := json.NewDecoder(bytes.NewReader(content))
	dec.DisallowUnknownFields()
	store := &okteto.ContextStore{}
	if err := dec.Decode(store); err != nil {
		return nil, err
	}
	if _, ok := store.Contexts[store.CurrentContext]; !ok {
		return nil, fmt.Errorf("the current context %q is not in the store", store.CurrentContext)
	}
	return store, nil
}

// certificateFor returns the certificate of the context of the instance in the store,
// which keys contexts by their URL with a scheme and without a trailing slash
func certificateFor(store *okteto.ContextStore, oktetoURL string) string {
	okCtx, ok := store.Contexts[okteto.NormalizeClusterURL(oktetoURL)]
	if !ok || okCtx == nil {
		return ""
	}
	return okCtx.Certificate
}
