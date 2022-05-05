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
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/k8s/labels"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

type proxyConfig struct {
	port  int
	token string
}

//Proxy refers to a proxy configuration
type Proxy struct {
	s            *http.Server
	proxyConfig  proxyConfig
	proxyHandler *proxyHandler
}

type proxyHandler struct {
	Name              string
	DivertedNamespace string
}

//NewProxy creates a new proxy
func NewProxy(kubeconfig *KubeConfig) (*Proxy, error) {
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

	ph := &proxyHandler{}
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
func (p *Proxy) SetName(name string) {
	p.proxyHandler.SetName(name)
}

// SetDivert sets the namespace used for divert
func (p *Proxy) SetDivert(divertedNamespace string) {
	p.proxyHandler.SetDivert(divertedNamespace)
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
			rw.WriteHeader(401)
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
				rw.WriteHeader(500)
				return
			}
			reverseProxy = httputil.NewSingleHostReverseProxy(destinationURL)
			reverseProxy.Transport = t
		}

		// Modify all resources updated or created to include the label.
		if r.Method == "PUT" || r.Method == "POST" {
			b, err := io.ReadAll(r.Body)
			if err != nil {
				oktetoLog.Infof("could not read the request body: %s", err)
				rw.WriteHeader(500)
				return
			}
			defer r.Body.Close()
			if len(b) == 0 {
				reverseProxy.ServeHTTP(rw, r)
				return
			}

			b, err = ph.translateBody(b)
			if err != nil {
				oktetoLog.Info(err)
				rw.WriteHeader(500)
				return
			}

			// Needed to set the new Content-Length
			r.ContentLength = int64(len(b))
			r.Body = io.NopCloser(bytes.NewBuffer(b))
		}

		// Redirect request to the k8s server (based on the transport HTTP generated from the config)
		reverseProxy.ServeHTTP(rw, r)
	})

	return handler, nil

}

func (ph *proxyHandler) SetName(name string) {
	ph.Name = name
}

func (ph *proxyHandler) SetDivert(divertedNamespace string) {
	ph.DivertedNamespace = divertedNamespace
}

func (ph *proxyHandler) translateBody(b []byte) ([]byte, error) {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(b, &body); err != nil {
		return nil, fmt.Errorf("could not unmarshal request: %s", err)
	}

	if err := ph.translateMetadata(body); err != nil {
		return nil, err
	}

	var typeMeta metav1.TypeMeta
	if err := json.Unmarshal(b, &typeMeta); err != nil {
		return nil, fmt.Errorf("could not process resource's type: %s", err)
	}

	switch typeMeta.Kind {
	case "Deployment":
		if err := ph.translateDeploymentSpec(body); err != nil {
			return nil, err
		}
	case "StatefulSet":
		if err := ph.translateStatefulSetSpec(body); err != nil {
			return nil, err
		}
	case "Job":
		if err := ph.translateJobSpec(body); err != nil {
			return nil, err
		}
	case "CronJob":
		if err := ph.translateCronJobSpec(body); err != nil {
			return nil, err
		}
	case "DaemonSet":
		if err := ph.translateDaemonSetSpec(body); err != nil {
			return nil, err
		}
	case "ReplicationController":
		if err := ph.translateReplicationControllerSpec(body); err != nil {
			return nil, err
		}
	case "ReplicaSet":
		if err := ph.translateReplicaSetSpec(body); err != nil {
			return nil, err
		}
	}

	return json.Marshal(body)
}

func (ph *proxyHandler) translateMetadata(body map[string]json.RawMessage) error {
	m, ok := body["metadata"]
	if !ok {
		return fmt.Errorf("request body doesn't have metadata field")
	}

	var metadata metav1.ObjectMeta
	if err := json.Unmarshal(m, &metadata); err != nil {
		return fmt.Errorf("could not process resource's metadata: %s", err)
	}

	labels.SetInMetadata(&metadata, model.DeployedByLabel, ph.Name)

	if metadata.Annotations == nil {
		metadata.Annotations = map[string]string{}
	}
	if utils.IsOktetoRepo() {
		metadata.Annotations[model.OktetoSampleAnnotation] = "true"
	}

	metadataAsByte, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("could not process resource's metadata: %s", err)
	}

	body["metadata"] = metadataAsByte

	return nil
}

func (ph *proxyHandler) translateDeploymentSpec(body map[string]json.RawMessage) error {
	var spec appsv1.DeploymentSpec
	if err := json.Unmarshal(body["spec"], &spec); err != nil {
		return fmt.Errorf("could not process deployment spec: %s", err)
	}
	labels.SetInMetadata(&spec.Template.ObjectMeta, model.DeployedByLabel, ph.Name)
	ph.applyDivert(&spec.Template.Spec)
	specAsByte, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not process deployment's spec: %s", err)
	}
	body["spec"] = specAsByte
	return nil
}

func (ph *proxyHandler) translateStatefulSetSpec(body map[string]json.RawMessage) error {
	var spec appsv1.StatefulSetSpec
	if err := json.Unmarshal(body["spec"], &spec); err != nil {
		return fmt.Errorf("could not process statefulset spec: %s", err)
	}
	labels.SetInMetadata(&spec.Template.ObjectMeta, model.DeployedByLabel, ph.Name)
	ph.applyDivert(&spec.Template.Spec)
	specAsByte, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not process statefulset's spec: %s", err)
	}
	body["spec"] = specAsByte
	return nil
}

func (ph *proxyHandler) translateJobSpec(body map[string]json.RawMessage) error {
	var spec batchv1.JobSpec
	if err := json.Unmarshal(body["spec"], &spec); err != nil {
		return fmt.Errorf("could not process job spec: %s", err)
	}
	labels.SetInMetadata(&spec.Template.ObjectMeta, model.DeployedByLabel, ph.Name)
	ph.applyDivert(&spec.Template.Spec)
	specAsByte, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not process job's spec: %s", err)
	}
	body["spec"] = specAsByte
	return nil
}

func (ph *proxyHandler) translateCronJobSpec(body map[string]json.RawMessage) error {
	var spec batchv1.CronJobSpec
	if err := json.Unmarshal(body["spec"], &spec); err != nil {
		return fmt.Errorf("could not process cronjob spec: %s", err)
	}
	labels.SetInMetadata(&spec.JobTemplate.Spec.Template.ObjectMeta, model.DeployedByLabel, ph.Name)
	ph.applyDivert(&spec.JobTemplate.Spec.Template.Spec)
	specAsByte, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not process cronjob's spec: %s", err)
	}
	body["spec"] = specAsByte
	return nil
}

func (ph *proxyHandler) translateDaemonSetSpec(body map[string]json.RawMessage) error {
	var spec appsv1.DaemonSetSpec
	if err := json.Unmarshal(body["spec"], &spec); err != nil {
		return fmt.Errorf("could not process daemonset spec: %s", err)
	}
	labels.SetInMetadata(&spec.Template.ObjectMeta, model.DeployedByLabel, ph.Name)
	ph.applyDivert(&spec.Template.Spec)
	specAsByte, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not process daemonset's spec: %s", err)
	}
	body["spec"] = specAsByte
	return nil
}

func (ph *proxyHandler) translateReplicationControllerSpec(body map[string]json.RawMessage) error {
	var spec apiv1.ReplicationControllerSpec
	if err := json.Unmarshal(body["spec"], &spec); err != nil {
		return fmt.Errorf("could not process replicationcontroller spec: %s", err)
	}
	labels.SetInMetadata(&spec.Template.ObjectMeta, model.DeployedByLabel, ph.Name)
	ph.applyDivert(&spec.Template.Spec)
	specAsByte, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not process replicationcontroller's spec: %s", err)
	}
	body["spec"] = specAsByte
	return nil
}

func (ph *proxyHandler) translateReplicaSetSpec(body map[string]json.RawMessage) error {
	var spec appsv1.ReplicaSetSpec
	if err := json.Unmarshal(body["spec"], &spec); err != nil {
		return fmt.Errorf("could not process replicaset spec: %s", err)
	}
	labels.SetInMetadata(&spec.Template.ObjectMeta, model.DeployedByLabel, ph.Name)
	ph.applyDivert(&spec.Template.Spec)
	specAsByte, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not process replicaset's spec: %s", err)
	}
	body["spec"] = specAsByte
	return nil
}

func (ph *proxyHandler) applyDivert(podSpec *apiv1.PodSpec) {
	if ph.DivertedNamespace == "" {
		return
	}
	if podSpec.DNSConfig == nil {
		podSpec.DNSConfig = &apiv1.PodDNSConfig{}
	}
	if podSpec.DNSConfig.Searches == nil {
		podSpec.DNSConfig.Searches = []string{}
	}
	searches := []string{fmt.Sprintf("%s.svc.cluster.local", ph.DivertedNamespace)}
	searches = append(searches, podSpec.DNSConfig.Searches...)
	podSpec.DNSConfig.Searches = searches
}
