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

package deployable

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/google/uuid"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/divert"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/runtime/serializer/protobuf"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/scheme"
)

const (
	headerUpgrade = "Upgrade"
)

type proxyConfig struct {
	token string
	port  int
}

// Proxy refers to a proxy configuration
type Proxy struct {
	s            *http.Server
	proxyHandler *proxyHandler
	proxyConfig  proxyConfig
}

type proxyHandler struct {
	DivertDriver divert.Driver
	// Name is sanitized version of the pipeline name
	Name             string
	translator       *Translator
	analyticsTracker *analytics.Tracker
}

// NewProxy creates a new proxy
func NewProxy(kubeconfig KubeConfigHandler, portGetter PortGetterFunc) (*Proxy, error) {
	// Look for a free local port to start the proxy
	port, err := portGetter("localhost")
	if err != nil {
		if dnsError, ok := err.(*net.DNSError); ok && dnsError.IsNotFound {
			return nil, oktetoErrors.UserError{
				E:    fmt.Errorf("could not find available ports: %w", dnsError),
				Hint: "Review your configuration to make sure 'localhost' is resolved correctly",
			}
		}
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

	ph := &proxyHandler{
		analyticsTracker: analytics.NewAnalyticsTracker(),
	}
	handler, err := ph.getProxyHandler(sessionToken, clusterConfig)
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
		ReadTimeout:  okteto.GetKubernetesTimeout(),
		WriteTimeout: okteto.GetKubernetesTimeout(),
		IdleTimeout:  okteto.GetKubernetesTimeout(),
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
		s:            s,
		proxyHandler: ph,
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

// SetName sets the name to be in the deployed-by label
// name is sanitized when passing the parameter
func (p *Proxy) SetName(name string) {
	p.proxyHandler.SetName(name)
}

// SetDivert sets the divert driver
func (p *Proxy) SetDivert(driver divert.Driver) {
	p.proxyHandler.SetDivert(driver)
}

func (p *Proxy) InitTranslator() {
	p.proxyHandler.translator = newTranslator(p.proxyHandler.Name, p.proxyHandler.DivertDriver)
}

// shouldInterceptRequest returns true if the request should be intercepted to inject labels and transformations.
// PUT and POST requests are always intercepted.
// PATCH requests are only intercepted for server-side apply operations to avoid issues with partial objects.
func shouldInterceptRequest(r *http.Request) bool {
	if r.Method == "PUT" || r.Method == "POST" {
		return true
	}
	// For PATCH, only intercept server-side apply operations
	if r.Method == "PATCH" && r.Header.Get("Content-Type") == "application/apply-patch+yaml" {
		return true
	}
	return false
}

// decodeObject decodes the bytes into a runtime.Object
// It first tries to decode as a typed object using UniversalDeserializer
// If that fails (e.g., for CRDs), it falls back to decoding as unstructured
func (ph *proxyHandler) decodeObject(b []byte, contentType string) (runtime.Object, error) {
	// First attempt: decode as typed object (works for standard K8s resources)
	decoder := scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode(b, nil, nil)
	if err == nil {
		return obj, nil
	}

	// Fallback: decode as unstructured (works for CRDs and other unknown types)
	oktetoLog.Debugf("error decoding as typed resource, trying unstructured: %s", err.Error())

	unstructuredObj := &unstructured.Unstructured{}
	var unstructuredDecoder runtime.Decoder

	switch contentType {
	case "application/vnd.kubernetes.protobuf":
		unstructuredDecoder = protobuf.NewSerializer(scheme.Scheme, scheme.Scheme)
	default:
		// JSON/YAML decoder (JSON and YAML are interchangeable for Kubernetes)
		unstructuredDecoder = json.NewSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, false)
	}

	_, _, err = unstructuredDecoder.Decode(b, nil, unstructuredObj)
	if err != nil {
		return nil, fmt.Errorf("error decoding object: %w", err)
	}

	return unstructuredObj, nil
}

func (ph *proxyHandler) getEncoder(contentType string) runtime.Encoder {
	switch contentType {
	case "application/vnd.kubernetes.protobuf":
		return protobuf.NewSerializer(scheme.Scheme, scheme.Scheme)
	case "application/json", "application/apply-patch+yaml":
		return json.NewSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, false)
	default:
		return json.NewSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, false)
	}
}

func (ph *proxyHandler) getProxyHandler(token string, clusterConfig *rest.Config) (http.Handler, error) {
	// By default we don't disable HTTP/2
	trans, err := newProtocolTransport(clusterConfig, false)
	if err != nil {
		oktetoLog.Infof("could not get http transport from config: %s", err)
		return nil, err
	}

	handler := http.NewServeMux()

	destinationURL := &url.URL{
		Host:   strings.TrimPrefix(clusterConfig.Host, "https://"),
		Scheme: "https",
	}
	proxy := httputil.NewSingleHostReverseProxy(destinationURL)
	proxy.Transport = trans

	oktetoLog.Debugf("forwarding host: %s", clusterConfig.Host)

	handler.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		requestToken := r.Header.Get("Authorization")
		expectedToken := fmt.Sprintf("Bearer %s", token)
		// Validate token with the generated for the local kubeconfig file
		if requestToken != expectedToken {
			rw.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Set the right bearer token based on the original kubeconfig. Authorization header should not be sent
		// if clusterConfig.BearerToken is empty
		if clusterConfig.BearerToken != "" {
			r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", clusterConfig.BearerToken))
		} else {
			r.Header.Del("Authorization")
		}

		reverseProxy := proxy
		if isSPDY(r) {
			oktetoLog.Debugf("detected SPDY request, disabling HTTP/2 for request %s %s", r.Method, r.URL.String())
			// In case of a SPDY request, we create a new proxy with HTTP/2 disabled
			t, err := newProtocolTransport(clusterConfig, true)
			if err != nil {
				oktetoLog.Infof("could not disabled HTTP/2: %s", err)
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}
			reverseProxy = httputil.NewSingleHostReverseProxy(destinationURL)
			reverseProxy.Transport = t
		}

		r.Host = destinationURL.Host
		// Modify all resources updated or created to include the label.
		if shouldInterceptRequest(r) {
			b, err := io.ReadAll(r.Body)
			if err != nil {
				oktetoLog.Infof("could not read the request body: %s", err)
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}
			defer r.Body.Close()
			if len(b) == 0 {
				reverseProxy.ServeHTTP(rw, r)
				return
			}

			// Decode the request body into a runtime.Object
			// This automatically falls back to unstructured for CRDs
			contentType := r.Header.Get("Content-Type")
			obj, err := ph.decodeObject(b, contentType)
			if err != nil {
				oktetoLog.Infof("error decoding resource on proxy: %s", err.Error())
				// Restore the request body for the reverse proxy since we already consumed it
				r.Body = io.NopCloser(bytes.NewReader(b))
				r.ContentLength = int64(len(b))
				reverseProxy.ServeHTTP(rw, r)
				return
			}

			// Modify the object (works for both typed and unstructured objects)
			if err := ph.translator.Modify(obj); err != nil {
				oktetoLog.Info(err)
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}

			// Encode back to the same format based on Content-Type
			encoder := ph.getEncoder(contentType)
			var buf bytes.Buffer
			if err := encoder.Encode(obj, &buf); err != nil {
				oktetoLog.Infof("error encoding resource on proxy: %s", err.Error())
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}

			// Needed to set the new Content-Length
			r.ContentLength = int64(buf.Len())
			r.Body = io.NopCloser(&buf)
		}

		// Redirect request to the k8s server (based on the transport HTTP generated from the config)
		reverseProxy.ServeHTTP(rw, r)
	})

	return handler, nil

}

func (ph *proxyHandler) SetName(name string) {
	ph.Name = name
}

func (ph *proxyHandler) SetDivert(driver divert.Driver) {
	ph.DivertDriver = driver
}

func newProtocolTransport(clusterConfig *rest.Config, disableHTTP2 bool) (http.RoundTripper, error) {
	copiedConfig := &rest.Config{}
	*copiedConfig = *clusterConfig

	if disableHTTP2 {
		// According to https://pkg.go.dev/k8s.io/client-go/rest#TLSClientConfig, this is the way to disable HTTP/2
		copiedConfig.TLSClientConfig.NextProtos = []string{"http/1.1"}
	}

	return rest.TransportFor(copiedConfig)
}

func isSPDY(r *http.Request) bool {
	return strings.HasPrefix(strings.ToLower(r.Header.Get(headerUpgrade)), "spdy/")
}
