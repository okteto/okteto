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

package okteto

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoHttp "github.com/okteto/okteto/pkg/http"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/types"
	"github.com/shurcooL/graphql"
	"golang.org/x/oauth2"
)

// OktetoClient implementation to connect to Okteto API
type OktetoClient struct {
	client graphqlClientInterface

	namespace types.NamespaceInterface
	user      types.UserInterface
	preview   types.PreviewInterface
	pipeline  types.PipelineInterface
	stream    types.StreamInterface
	kubetoken types.KubetokenInterface
}

type OktetoClientProvider struct{}

var insecureSkipTLSVerify bool
var serverName string
var strictTLSOnce sync.Once
var errURLNotSet = errors.New("the okteto URL is not set")

// graphqlClientInterface contains the functions that a graphqlClient must have
type graphqlClientInterface interface {
	Query(ctx context.Context, q interface{}, variables map[string]interface{}) error
	Mutate(ctx context.Context, m interface{}, variables map[string]interface{}) error
}

func NewOktetoClientProvider() *OktetoClientProvider {
	return &OktetoClientProvider{}
}

func (*OktetoClientProvider) Provide(opts ...Option) (types.OktetoInterface, error) {
	c, err := NewOktetoClient(opts...)
	if err != nil {
		return nil, err
	}
	return c, err
}

type oktetoClientCfg struct {
	ctxName string
	token   string
}

func WithCtxName(ctxName string) Option {
	return func(cfg *oktetoClientCfg) {
		cfg.ctxName = ctxName
	}
}

func WithToken(token string) Option {
	return func(cfg *oktetoClientCfg) {
		cfg.token = token
	}
}

func defaultOktetoClientCfg() *oktetoClientCfg {
	if CurrentStore == nil && !ContextExists() {
		return &oktetoClientCfg{}
	}
	if CurrentStore != nil && CurrentStore.CurrentContext == "" {
		return &oktetoClientCfg{}
	}
	return &oktetoClientCfg{
		ctxName: Context().Name,
		token:   Context().Token,
	}

}

type Option func(*oktetoClientCfg)

// NewOktetoClient creates a new client to connect with Okteto API
func NewOktetoClient(opts ...Option) (*OktetoClient, error) {
	cfg := defaultOktetoClientCfg()
	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.token == "" {
		okCtx, exists := ContextStore().Contexts[cfg.ctxName]
		if !exists {
			return nil, fmt.Errorf("%s context doesn't exists", cfg.ctxName)
		}
		cfg.token = okCtx.Token
	}

	httpClient, u, err := newOktetoHttpClient(cfg.ctxName, cfg.token, "graphql")
	if err != nil {
		return nil, err
	}

	return newOktetoClientFromGraphqlClient(u, httpClient)
}

func newOktetoHttpClient(contextName, token, oktetoUrlPath string) (*http.Client, string, error) {
	if token == "" {
		return nil, "", fmt.Errorf(oktetoErrors.ErrNotLogged, contextName)
	}
	u, err := parseOktetoURLWithPath(contextName, oktetoUrlPath)
	if err != nil {
		return nil, "", err
	}

	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token,
			TokenType: "Bearer"},
	)

	sslTransportOption := &oktetoHttp.SSLTransportOption{}

	if serverName != "" {
		sslTransportOption.ServerName = serverName
		sslTransportOption.URLsToIntercept = []string{u}
	}

	ctxHttpClient := oktetoHttp.StrictSSLHTTPClient(sslTransportOption)

	if insecureSkipTLSVerify {
		ctxHttpClient = oktetoHttp.InsecureHTTPClient()
	} else if cert, err := GetContextCertificate(); err == nil {
		sslTransportOption.Certs = []*x509.Certificate{cert}
		ctxHttpClient = oktetoHttp.StrictSSLHTTPClient(sslTransportOption)
	}

	ctx := contextWithOauth2HttpClient(context.Background(), ctxHttpClient)

	httpClient := oauth2.NewClient(ctx, src)

	return httpClient, u, err
}

// NewOktetoClientFromUrlAndToken creates a new client to connect with Okteto API provided url and token
func NewOktetoClientFromUrlAndToken(url, token string) (*OktetoClient, error) {
	u, err := parseOktetoURL(url)
	if err != nil {
		return nil, fmt.Errorf("could not create okteto client: %w", err)
	}

	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token,
			TokenType: "Bearer"},
	)

	sslTransportOption := &oktetoHttp.SSLTransportOption{}

	if serverName != "" {
		sslTransportOption.ServerName = serverName
		sslTransportOption.URLsToIntercept = []string{u}
	}

	ctxHttpClient := oktetoHttp.StrictSSLHTTPClient(sslTransportOption)

	if insecureSkipTLSVerify {
		ctxHttpClient = oktetoHttp.InsecureHTTPClient()
	} else if cert, err := GetContextCertificate(); err == nil {
		sslTransportOption.Certs = []*x509.Certificate{cert}
		ctxHttpClient = oktetoHttp.StrictSSLHTTPClient(sslTransportOption)
	}

	ctx := contextWithOauth2HttpClient(context.Background(), ctxHttpClient)

	httpClient := oauth2.NewClient(ctx, src)

	return newOktetoClientFromGraphqlClient(u, httpClient)
}

// NewOktetoClientFromUrl creates a new client to connect with Okteto API provided an url
func NewOktetoClientFromUrl(url string) (*OktetoClient, error) {
	u, err := parseOktetoURL(url)
	if err != nil {
		return nil, err
	}

	sslTransportOption := &oktetoHttp.SSLTransportOption{}

	if serverName != "" {
		sslTransportOption.ServerName = serverName
		sslTransportOption.URLsToIntercept = []string{u}
	}

	ctxHttpClient := oktetoHttp.StrictSSLHTTPClient(sslTransportOption)

	if insecureSkipTLSVerify {
		ctxHttpClient = oktetoHttp.InsecureHTTPClient()
	} else if cert, err := GetContextCertificate(); err == nil {
		sslTransportOption.Certs = []*x509.Certificate{cert}
		ctxHttpClient = oktetoHttp.StrictSSLHTTPClient(sslTransportOption)
	}

	ctx := contextWithOauth2HttpClient(context.Background(), ctxHttpClient)

	httpClient := oauth2.NewClient(ctx, nil)

	return newOktetoClientFromGraphqlClient(u, httpClient)
}

// contextWithOauth2HttpClient returns a context.Context with a value of type oauth2.HTTPClient so oauth2.NewClient() can be bootstrapped with a custom http.Client
func contextWithOauth2HttpClient(ctx context.Context, httpClient *http.Client) context.Context {
	return context.WithValue(
		ctx,
		oauth2.HTTPClient,
		httpClient,
	)
}

func newOktetoClientFromGraphqlClient(url string, httpClient *http.Client) (*OktetoClient, error) {
	c := &OktetoClient{
		client: graphql.NewClient(url, httpClient),
	}
	c.namespace = newNamespaceClient(c.client)
	c.preview = newPreviewClient(c.client)
	c.user = newUserClient(c.client)
	c.pipeline = newPipelineClient(c.client, url)
	c.stream = newStreamClient(httpClient)
	c.kubetoken = newKubeTokenClient(httpClient)
	return c, nil
}

func parseOktetoURL(u string) (string, error) {
	return parseOktetoURLWithPath(u, "graphql")
}

func parseOktetoURLWithPath(u, path string) (string, error) {
	if u == "" {
		return "", errURLNotSet
	}

	parsed, err := url.Parse(u)
	if err != nil {
		return "", err
	}

	if parsed.Scheme == "" {
		parsed.Scheme = "https"
		parsed.Host = parsed.Path
	}

	parsed.Path = path
	return parsed.String(), nil
}

func translateAPIErr(err error) error {
	e := strings.TrimPrefix(err.Error(), "graphql: ")
	switch e {
	case "not-authorized":
		return fmt.Errorf(oktetoErrors.ErrNotLogged, Context().Name)
	case "namespace-quota-exceeded":
		return fmt.Errorf("you have exceeded your namespace quota. Contact us at hello@okteto.com to learn more")
	case "namespace-quota-exceeded-onpremises":
		return fmt.Errorf("you have exceeded your namespace quota, please contact your administrator to increase it")
	case "users-limit-exceeded":
		return fmt.Errorf("license limit exceeded. Contact your administrator to update your license and try again")
	case "internal-server-error":
		return fmt.Errorf("server temporarily unavailable, please try again")
	case "non-200 OK status code: 401 Unauthorized body: \"\"":
		return fmt.Errorf("unauthorized. Please run 'okteto context url' and try again")
	case "not-found":
		return oktetoErrors.ErrNotFound

	default:
		switch {
		case oktetoErrors.IsX509(err):
			return oktetoErrors.UserError{
				E:    err,
				Hint: oktetoErrors.ErrX509Hint,
			}
		case isAPILicenseError(err):
			return oktetoErrors.UserError{
				E:    oktetoErrors.ErrInvalidLicense,
				Hint: "The Okteto instance for your current context has an expired or missing license. Please contact your administrator for more information",
			}
		}

		oktetoLog.Infof("Unrecognized API error: %s", err)
		return err
	}

}

func isAPILicenseError(err error) bool {
	return strings.HasPrefix(err.Error(), "non-200 OK status code: 423")
}

func isAPITransientErr(err error) bool {
	if err == nil {
		return false
	}

	switch {
	case
		strings.Contains(err.Error(), "can't assign requested address"),
		strings.Contains(err.Error(), "command exited without exit status or exit signal"),
		strings.Contains(err.Error(), "connection refused"),
		strings.Contains(err.Error(), "connection reset by peer"),
		strings.Contains(err.Error(), "client connection lost"),
		strings.Contains(err.Error(), "nodename nor servname provided, or not known"),
		strings.Contains(err.Error(), "unexpected EOF"),
		strings.Contains(err.Error(), "TLS handshake timeout"),
		strings.Contains(err.Error(), "broken pipe"),
		strings.Contains(err.Error(), "No connection could be made"),
		strings.Contains(err.Error(), "dial tcp: operation was canceled"),
		strings.Contains(err.Error(), "network is unreachable"),
		strings.Contains(err.Error(), "development container has been removed"):
		return true
	default:
		return false
	}

}

// InDevContainer returns true if running in an okteto dev container
func InDevContainer() bool {
	if v, ok := os.LookupEnv(constants.OktetoNameEnvVar); ok && v != "" {
		return true
	}

	return false
}

func query(ctx context.Context, query interface{}, variables map[string]interface{}, client graphqlClientInterface) error {
	err := client.Query(ctx, query, variables)
	if err != nil {
		if isAPITransientErr(err) {
			err = client.Query(ctx, query, variables)
		}
		if err != nil {
			return translateAPIErr(err)
		}
	}
	return nil
}

func mutate(ctx context.Context, mutation interface{}, variables map[string]interface{}, client graphqlClientInterface) error {
	err := client.Mutate(ctx, mutation, variables)
	if err != nil {
		return translateAPIErr(err)
	}
	return nil
}

// Namespaces retrieves the NamespaceClient
func (c *OktetoClient) Namespaces() types.NamespaceInterface {
	return c.namespace
}

// Previews retrieves the Previews client
func (c *OktetoClient) Previews() types.PreviewInterface {
	return c.preview
}

// Pipeline retrieves the Pipeline client
func (c *OktetoClient) Pipeline() types.PipelineInterface {
	return c.pipeline
}

// User retrieves the UserClient
func (c *OktetoClient) User() types.UserInterface {
	return c.user
}

// Stream retrieves the Stream client
func (c *OktetoClient) Stream() types.StreamInterface {
	return c.stream
}

// Kubetoken retrieves the Kubetoken client
func (c *OktetoClient) Kubetoken() types.KubetokenInterface {
	return c.kubetoken
}

func SetInsecureSkipTLSVerifyPolicy(isInsecure bool) {
	oktetoLog.Debugf("insecure mode: %t", isInsecure)
	if isInsecure {
		oktetoLog.Warning("Insecure mode enabled")
	}
	insecureSkipTLSVerify = isInsecure
}

func IsInsecureSkipTLSVerifyPolicy() bool {
	return insecureSkipTLSVerify
}

func SetServerNameOverride(serverNameOverride string) {
	oktetoLog.Debugf("server name override: %q", serverNameOverride)
	if serverNameOverride == "" {
		return
	}
	host, port, err := net.SplitHostPort(serverNameOverride)
	if err != nil {
		oktetoLog.Fatalf("invalid server name %q: %s", serverNameOverride, err)
		return
	}
	oktetoLog.Warning("Server name overridden to host=%s port=%s", host, port)
	serverName = net.JoinHostPort(host, port)
}

func GetServerNameOverride() string {
	return serverName
}
