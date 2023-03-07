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
	"errors"
	"fmt"

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
}

// client connects with the
type client struct {
	config configInterface
	get    func(ref name.Reference, options ...remote.Option) (*remote.Descriptor, error)
}

func newOktetoRegistryClient(config configInterface) client {
	return client{
		config: config,
		get:    remote.Get,
	}
}

func (c client) getDescriptor(image string) (*remote.Descriptor, error) {
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

// GetDigest returns the digest of an image
func (c client) GetDigest(image string) (string, error) {
	descriptor, err := c.getDescriptor(image)
	if err != nil {
		return "", fmt.Errorf("error getting image digest: %w", err)
	}
	return descriptor.Digest.String(), nil
}

// GetImageConfig returns the config of an image
func (c client) GetImageConfig(image string) (*v1.ConfigFile, error) {
	descriptor, err := c.getDescriptor(image)
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
	return []remote.Option{c.getAuthentication(ref), c.getTransport()}
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
	return remote.WithAuthFromKeychain(authn.DefaultKeychain)
}

func (c client) getTransport() remote.Option {
	transport := oktetoHttp.DefaultTransport()

	if c.config.IsInsecureSkipTLSVerifyPolicy() {
		transport = oktetoHttp.InsecureTransport()
	} else if cert, err := c.config.GetContextCertificate(); err == nil {
		transport = oktetoHttp.StrictSSLTransport(cert)
	}
	return remote.WithTransport(transport)
}
