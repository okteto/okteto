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

package buildkit

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"

	"github.com/moby/buildkit/client"
	"github.com/okteto/okteto/pkg/log/io"
	"golang.org/x/oauth2"
	"google.golang.org/grpc/credentials/oauth"
)

const (
	// readWritePermission is the permission for reading and writing
	readWritePermission = 0600
)

// urlParser defines an interface for parsing URLs
type urlParser interface {
	Parse(rawurl string) (*url.URL, error)
}

// realURLParser implements the urlParser interface
type realURLParser struct{}

// Parse parses a URL
func (p *realURLParser) Parse(rawurl string) (*url.URL, error) {
	return url.Parse(rawurl)
}

// certDecoder defines an interface for decoding certificates
type certDecoder interface {
	DecodeString(s string) ([]byte, error)
}

// base64CertDecoder implements the certDecoder interface
type base64CertDecoder struct{}

// DecodeString decodes a base64 string
func (b *base64CertDecoder) DecodeString(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

type fileWriter interface {
	WriteFile(filename string, data []byte, perm os.FileMode) error
}

// osFileWriter implements the fileWriter interface
type osFileWriter struct{}

// WriteFile writes data to a file
func (o *osFileWriter) WriteFile(filename string, data []byte, perm os.FileMode) error {
	return os.WriteFile(filename, data, perm)
}

// buildkitClientCreator defines an interface for creating buildkit clients
type buildkitClientCreator interface {
	New(ctx context.Context, address string, opts ...client.ClientOpt) (*client.Client, error)
}

// realBuildkitClientCreator implements the buildkitClientCreator interface
type realBuildkitClientCreator struct{}

// New creates a new buildkit client
func (r *realBuildkitClientCreator) New(ctx context.Context, address string, opts ...client.ClientOpt) (*client.Client, error) {
	return client.New(ctx, address, opts...)
}

// ClientFactory is a factory for buildkit clients
type ClientFactory struct {
	logger                *io.Controller
	urlParser             urlParser
	buildkitClientCreator buildkitClientCreator
	certDecoder           certDecoder
	fileWriter            fileWriter
	cert                  string
	token                 string
	builder               string
	certificatePath       string
}

// NewBuildkitClientFactory creates a new buildkit client factory
func NewBuildkitClientFactory(cert, builder, token, certificatePath string, l *io.Controller) *ClientFactory {
	return &ClientFactory{
		cert:                  cert,
		token:                 token,
		builder:               builder,
		certificatePath:       certificatePath,
		urlParser:             &realURLParser{},
		certDecoder:           &base64CertDecoder{},
		fileWriter:            &osFileWriter{},
		buildkitClientCreator: &realBuildkitClientCreator{},
		logger:                l,
	}
}

// GetBuildkitClient returns a buildkit client
func (bcf *ClientFactory) GetBuildkitClient(ctx context.Context) (*client.Client, error) {
	bcf.logger.Logger().Infof("getting buildkit client")
	cert := bcf.cert

	if cert != "" {
		bcf.logger.Logger().Infof("using certificate from context")
		certBytes, err := bcf.certDecoder.DecodeString(cert)
		if err != nil {
			return nil, fmt.Errorf("certificate decoding error: %w", err)
		}

		if err := bcf.fileWriter.WriteFile(bcf.certificatePath, certBytes, readWritePermission); err != nil {
			return nil, fmt.Errorf("failed to write certificate: %w", err)
		}

		c, err := bcf.getClientWithCert(ctx)
		if err != nil {
			bcf.logger.Logger().Infof("failed to create okteto build client: %s", err)
			return nil, fmt.Errorf("failed to create the builder client: %w", err)
		}

		return c, nil
	}
	c, err := bcf.buildkitClientCreator.New(ctx, bcf.builder)
	if err != nil {
		return nil, fmt.Errorf("failed to create the builder client: %w", err)
	}
	return c, nil
}

// getClientWithCert returns a buildkit client with a certificate
func (bcf *ClientFactory) getClientWithCert(ctx context.Context) (*client.Client, error) {
	buildkitURL, err := bcf.urlParser.Parse(bcf.builder)
	if err != nil {
		return nil, fmt.Errorf("invalid buildkit host %s: %w", bcf.builder, err)
	}

	creds := client.WithCAAndSystemRoot(buildkitURL.Hostname(), bcf.certificatePath)
	oauthToken := &oauth2.Token{
		AccessToken: bcf.token,
	}

	rpc := client.WithRPCCreds(oauth.TokenSource{
		TokenSource: oauth2.StaticTokenSource(oauthToken),
	})

	c, err := bcf.buildkitClientCreator.New(ctx, bcf.builder, creds, rpc)
	if err != nil {
		return nil, fmt.Errorf("failed to create buildkit client: %w", err)
	}

	return c, nil
}
