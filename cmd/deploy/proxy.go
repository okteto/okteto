package deploy

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

type proxyConfig struct {
	port         int
	certificates []tls.Certificate
	token        string
}

type proxy struct {
	s           *http.Server
	proxyConfig proxyConfig
}

func newProxy(proxyConfig proxyConfig) *proxy {
	return &proxy{
		proxyConfig: proxyConfig,
	}
}

// Start starts the proxy server
func (p *proxy) Start(ctx context.Context, name string, clusterConfig *rest.Config) error {
	handler, err := p.createProxyHandler(ctx, name, clusterConfig)
	if err != nil {
		return err
	}

	p.createAndStartProxyServer(ctx, handler, p.proxyConfig.port, p.proxyConfig.certificates)

	return nil
}

func (p *proxy) createAndStartProxyServer(ctx context.Context, handler http.Handler, port int, certs []tls.Certificate) {
	s := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      handler,
		ReadTimeout:  3600 * time.Second,
		WriteTimeout: 3600 * time.Second,
		TLSConfig: &tls.Config{
			Certificates: certs,
		},
	}

	go func(s *http.Server) {
		log.Debugf("start server on %d", port)
		// Path to cert and key files are empty because cert is provisioned on the tls config struct
		if err := s.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Fatalf("could not start proxy server: %s", err)
		}
	}(s)

	p.s = s
}

func (p *proxy) createProxyHandler(ctx context.Context, name string, clusterConfig *rest.Config) (http.Handler, error) {
	trans, err := rest.TransportFor(clusterConfig)
	if err != nil {
		log.Errorf("could not get transport from config: %s", err)
		return nil, err
	}

	handler := http.DefaultServeMux

	proxy := httputil.NewSingleHostReverseProxy(&url.URL{
		Host:   strings.TrimPrefix(clusterConfig.Host, "https://"),
		Scheme: "https",
	})
	proxy.Transport = trans

	log.Debugf("forwarding host: %s", clusterConfig.Host)

	handler.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		expectedToken := fmt.Sprintf("Bearer %s", p.proxyConfig.token)
		// Validate token with the generated for the local kubeconfig file
		if token != expectedToken {
			rw.WriteHeader(401)
			return
		}

		// Set the right bearer token based on the original kubeconfig
		r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", clusterConfig.BearerToken))

		// Modify all resources updated or created to include the label. Probably this can be improved
		if r.Method == "PUT" || r.Method == "POST" {
			b, err := io.ReadAll(r.Body)
			if err != nil {
				log.Errorf("could not read the request body: %s", err)
				rw.WriteHeader(500)
				return
			}

			var body map[string]json.RawMessage
			if err := json.Unmarshal(b, &body); err != nil {
				log.Errorf("could not unmarshal request: %s", err)
				rw.WriteHeader(500)
				return
			}

			m, ok := body["metadata"]
			if !ok {
				log.Error("request body doesn't have metadata field")
				rw.WriteHeader(500)
				return
			}

			var metadata metav1.ObjectMeta
			if err := json.Unmarshal(m, &metadata); err != nil {
				log.Errorf("could not process resource's metadata: %s", err)
				rw.WriteHeader(500)
				return
			}

			if metadata.Labels == nil {
				metadata.Labels = map[string]string{}
			}
			metadata.Labels[model.DeployedByLabel] = name

			metadataAsByte, err := json.Marshal(metadata)
			if err != nil {
				log.Errorf("could not process resource's metadata: %s", err)
				rw.WriteHeader(500)
				return
			}

			body["metadata"] = metadataAsByte

			b, err = json.Marshal(body)
			if err != nil {
				log.Errorf("could not marshal modified body: %s", err)
				rw.WriteHeader(500)
				return
			}

			// Needed to set the new Content-Length
			r.ContentLength = int64(len(b))
			r.Body = io.NopCloser(bytes.NewBuffer(b))
		}

		// Redirect request to the k8s server (based on the transport HTTP generated from the config)
		proxy.ServeHTTP(rw, r)
	})

	return handler, nil
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
