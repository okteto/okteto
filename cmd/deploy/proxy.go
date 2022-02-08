// Copyright 2022 The Okteto Authors
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

package deploy

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

type proxyConfig struct {
	port  int
	token string
}

//Proxy refers to a proxy configuration
type Proxy struct {
	s           *http.Server
	proxyConfig proxyConfig
}

//NewProxy creates a new proxy
func NewProxy(name string, kubeconfig *KubeConfig) (*Proxy, error) {
	// Look for a free local port to start the proxy
	port, err := model.GetAvailablePort("localhost")
	if err != nil {
		oktetoLog.Errorf("could not find a free port to start proxy server: %s", err)
		return nil, err
	}
	oktetoLog.Debugf("found available port %d", port)

	// Generate a token for the requests done to the proxy
	sessionToken := uuid.NewString()

	clusterConfig, err := kubeconfig.Read()
	if err != nil {
		oktetoLog.Errorf("could not read kubeconfig file: %s", err)
		return nil, err
	}

	handler, err := getProxyHandler(name, sessionToken, clusterConfig)
	if err != nil {
		oktetoLog.Errorf("could not configure local proxy: %s", err)
		return nil, err
	}

	// TODO for now, using self-signed certificates
	cert, err := tls.X509KeyPair(cert, key)
	if err != nil {
		oktetoLog.Errorf("could not read certificate: %s", err)
		return nil, err
	}

	s := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},

			// Recommended security configuration by DeepSource
			MinVersion: tls.VersionTLS12,
			MaxVersion: tls.VersionTLS13,
		},
	}
	return &Proxy{
		proxyConfig: proxyConfig{
			port:  port,
			token: sessionToken,
		},
		s: s,
	}, nil
}

// Start starts the proxy server
func (p *Proxy) Start() {
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGTERM, syscall.SIGINT)
		<-sigint
		if p.s == nil {
			return
		}

		oktetoLog.Debugf("os.Interrupt - closing...")
		p.s.Close()
	}()

	go func(s *http.Server) {
		// Path to cert and key files are empty because cert is provisioned on the tls config struct
		if err := s.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			oktetoLog.Infof("could not start proxy server: %s", err)
		}
	}(p.s)
}

// Shutdown stops the proxy server
func (p *Proxy) Shutdown(ctx context.Context) error {
	if p.s == nil {
		return nil
	}

	return p.s.Shutdown(ctx)
}

// GetPort retrieves the port configured for the proxy
func (p *Proxy) GetPort() int {
	return p.proxyConfig.port
}

// GetToken Retrieves the token configured for the proxy
func (p *Proxy) GetToken() string {
	return p.proxyConfig.token
}
