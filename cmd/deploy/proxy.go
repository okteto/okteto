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

package deploy

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/okteto/okteto/pkg/log"
)

type proxyConfig struct {
	port  int
	token string
}

type proxy struct {
	s           *http.Server
	proxyConfig proxyConfig
}

func newProxy(proxyConfig proxyConfig, s *http.Server) *proxy {
	return &proxy{
		proxyConfig: proxyConfig,
		s:           s,
	}
}

// Start starts the proxy server
func (p *proxy) Start() {
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, syscall.SIGTERM, syscall.SIGINT)
		<-sigint
		if p.s == nil {
			return
		}

		log.Debugf("os.Interrupt - closing...")
		p.s.Close()
	}()

	go func(s *http.Server) {
		// Path to cert and key files are empty because cert is provisioned on the tls config struct
		if err := s.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Errorf("could not start proxy server: %s", err)
		}
	}(p.s)
}

// Shutdown stops the proxy server
func (p *proxy) Shutdown(ctx context.Context) error {
	if p.s == nil {
		return nil
	}

	return p.s.Shutdown(ctx)
}

// GetPort retrieves the port configured for the proxy
func (p *proxy) GetPort() int {
	return p.proxyConfig.port
}

// GetToken Retrieves the token configured for the proxy
func (p *proxy) GetToken() string {
	return p.proxyConfig.token
}
