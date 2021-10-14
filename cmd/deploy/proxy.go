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
