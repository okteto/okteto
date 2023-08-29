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

package registry

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoHttp "github.com/okteto/okteto/pkg/http"
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

type clientInterface interface {
	GetDigest(image string) (string, error)
	GetImageConfig(image string) (*v1.ConfigFile, error)
	HasPushAccess(image string) (bool, error)
	GetDescriptor(image string) (*remote.Descriptor, error)
	Write(ref name.Reference, image v1.Image) error
}

type ClientConfigInterface interface {
	GetRegistryURL() string
	GetUserID() string
	GetToken() string
	IsInsecureSkipTLSVerifyPolicy() bool
	GetContextCertificate() (*x509.Certificate, error)
	GetServerNameOverride() string
	GetContextName() string
	GetExternalRegistryCredentials(registryHost string) (string, string, error)
}

type oktetoHelperConfig interface {
	GetUserID() string
	GetToken() string
}

type oktetoHelper struct {
	config oktetoHelperConfig
}

func newOktetoHelper(config oktetoHelperConfig) oktetoHelper {
	return oktetoHelper{
		config: config,
	}
}

func (oh oktetoHelper) Get(_ string) (string, string, error) {
	return oh.config.GetUserID(), oh.config.GetToken(), nil
}

// client operates with the registry API
type client struct {
	config  ClientConfigInterface
	get     func(ref name.Reference, options ...remote.Option) (*remote.Descriptor, error)
	write   func(ref name.Reference, image v1.Image, options ...remote.Option) error
	tlsDial oktetoHttp.TLSDialFunc
}

func newOktetoRegistryClient(config ClientConfigInterface) client {
	return client{
		config:  config,
		get:     remote.Get,
		write:   remote.Write,
		tlsDial: oktetoHttp.DefaultTLSDial,
	}
}

// GetDescriptor returns the descriptor of an image
func (c client) GetDescriptor(image string) (*remote.Descriptor, error) {
	ref, err := name.ParseReference(image)
	if err != nil {
		return nil, err
	}

	options := c.getOptions(ref)

	descriptor, err := c.get(ref, options...)
	if err != nil {
		if c.isNotFound(err) {
			return nil, fmt.Errorf("error getting image descriptor: %w", oktetoErrors.ErrNotFound)
		}
		return nil, fmt.Errorf("error getting image descriptor: %w", err)
	}
	return descriptor, nil
}

// Write writes an image metadata to the registry
func (c client) Write(ref name.Reference, image v1.Image) error {
	options := c.getOptions(ref)
	return c.write(ref, image, options...)
}

// GetDigest returns the digest of an image
func (c client) GetDigest(image string) (string, error) {
	descriptor, err := c.GetDescriptor(image)
	if err != nil {
		return "", fmt.Errorf("error getting image digest: %w", err)
	}
	return descriptor.Digest.String(), nil
}

// GetImageConfig returns the config of an image
func (c client) GetImageConfig(image string) (*v1.ConfigFile, error) {
	descriptor, err := c.GetDescriptor(image)
	if err != nil {
		return nil, fmt.Errorf("error getting image configuration: %w", err)
	}

	img, err := descriptor.Image()
	if err != nil {
		return nil, fmt.Errorf("error getting image configuration: %w", err)
	}
	cfg, err := img.ConfigFile()
	if err != nil {
		return nil, fmt.Errorf("error getting image configuration: %w", err)
	}
	return cfg, nil
}

func (c client) HasPushAccess(image string) (bool, error) {
	ref, err := name.ParseReference(image)
	if err != nil {
		return false, fmt.Errorf("error checking push access: %w", err)
	}
	err = remote.CheckPushPermission(ref, c.getAuthHelper(ref), c.getTransport())
	return err == nil, err
}

func (c client) isNotFound(err error) bool {
	var transportErr *transport.Error
	if errors.As(err, &transportErr) {
		for _, err := range transportErr.Errors {
			if err.Code == transport.ManifestUnknownErrorCode {
				return true
			}
		}
	}
	return false
}

func (c client) getOptions(ref name.Reference) []remote.Option {
	return []remote.Option{c.getAuthentication(ref), c.getTransportOption()}
}

func (c client) getAuthHelper(_ name.Reference) authn.Keychain {
	helper := newOktetoHelper(c.config)
	return authn.NewKeychainFromHelper(helper)
}

func (c client) getAuthentication(ref name.Reference) remote.Option {
	registry := ref.Context().RegistryStr()
	oktetoLog.Debugf("calling registry %s", registry)

	okRegistry := c.config.GetRegistryURL()
	if okRegistry == registry {
		authenticator := &authn.Basic{
			Username: c.config.GetUserID(),
			Password: c.config.GetToken(),
		}
		return remote.WithAuth(authenticator)
	}

	kc := authn.NewMultiKeychain(
		authn.DefaultKeychain,
		authn.NewKeychainFromHelper(inlineHelper(c.config.GetExternalRegistryCredentials)),
	)

	return remote.WithAuthFromKeychain(kc)
}

func (c client) getTransportOption() remote.Option {
	return remote.WithTransport(c.getTransport())
}
func (c client) getTransport() http.RoundTripper {
	sslTransportOption := &oktetoHttp.SSLTransportOption{
		TLSDial: c.tlsDial,
	}

	if serverName := c.config.GetServerNameOverride(); serverName != "" {
		sslTransportOption.ServerName = serverName
		sslTransportOption.URLsToIntercept = []string{
			"//" + c.config.GetRegistryURL(),
			c.config.GetContextName(),
		}
	}

	transport := oktetoHttp.StrictSSLTransport(sslTransportOption)

	if c.config.IsInsecureSkipTLSVerifyPolicy() {
		transport = oktetoHttp.InsecureTransport()
	} else if cert, err := c.config.GetContextCertificate(); err == nil {
		sslTransportOption.Certs = []*x509.Certificate{cert}
		transport = oktetoHttp.StrictSSLTransport(sslTransportOption)
	}
	return transport
}

type inlineHelper func(registryURL string) (string, string, error)

func (fn inlineHelper) Get(registryURL string) (string, string, error) {
	return fn(registryURL)
}
