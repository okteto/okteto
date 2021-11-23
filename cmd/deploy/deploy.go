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
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/endpoints"
	"github.com/okteto/okteto/pkg/k8s/ingresses"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/replicasets"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
)

const headerUpgrade = "Upgrade"

var tempKubeConfigTemplate = "%s/.okteto/kubeconfig-%s"

// Options options for deploy command
type Options struct {
	ManifestPath string
	Name         string
	Variables    []string
	Wait         bool
	Timeout      time.Duration
}

type kubeConfigHandler interface {
	Read() (*rest.Config, error)
	Modify(port int, sessionToken, destKubeconfigFile string) error
}

type proxyInterface interface {
	Start()
	Shutdown(ctx context.Context) error
	GetPort() int
	GetToken() string
}

type deployCommand struct {
	getManifest func(cwd, name, filename string) (*utils.Manifest, error)

	proxy              proxyInterface
	kubeconfig         kubeConfigHandler
	executor           utils.ManifestExecutor
	tempKubeconfigFile string
}

//Deploy deploys the okteto manifest
func Deploy(ctx context.Context) *cobra.Command {
	options := &Options{}

	cmd := &cobra.Command{
		Use:    "deploy",
		Short:  "Execute the list of commands specified in the Okteto manifest to deploy the application",
		Args:   utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#version"),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// This is needed because the deploy command needs the original kubeconfig configuration even in the execution within another
			// deploy command. If not, we could be proxying a proxy and we would be applying the incorrect deployed-by label
			os.Setenv("OKTETO_WITHIN_DEPLOY_COMMAND_CONTEXT", "false")
			if err := contextCMD.Run(ctx, &contextCMD.ContextOptions{}); err != nil {
				return err
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get the current working directory: %w", err)
			}

			if options.Name == "" {
				options.Name = utils.InferApplicationName(cwd)
			}

			// Look for a free local port to start the proxy
			port, err := model.GetAvailablePort("localhost")
			if err != nil {
				log.Errorf("could not find a free port to start proxy server: %s", err)
				return err
			}
			log.Debugf("found available port %d", port)

			// TODO for now, using self-signed certificates
			cert, err := tls.X509KeyPair(cert, key)
			if err != nil {
				log.Errorf("could not read certificate: %s", err)
				return err
			}

			// Generate a token for the requests done to the proxy
			sessionToken := uuid.NewString()

			kubeconfig := newKubeConfig()
			clusterConfig, err := kubeconfig.Read()
			if err != nil {
				log.Errorf("could not read kubeconfig file: %s", err)
				return err
			}

			handler, err := getProxyHandler(options.Name, sessionToken, clusterConfig)
			if err != nil {
				log.Errorf("could not configure local proxy: %s", err)
				return err
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

			c := &deployCommand{
				getManifest: utils.GetManifest,

				kubeconfig: kubeconfig,
				executor:   utils.NewExecutor(),
				proxy: newProxy(proxyConfig{
					port:  port,
					token: sessionToken,
				}, s),
				tempKubeconfigFile: fmt.Sprintf(tempKubeConfigTemplate, config.GetUserHomeDir(), options.Name),
			}
			return c.runDeploy(ctx, cwd, options)
		},
	}

	cmd.Flags().StringVar(&options.Name, "name", "", "application name")
	cmd.Flags().StringVarP(&options.ManifestPath, "file", "f", "", "path to the manifest file")
	cmd.Flags().StringArrayVarP(&options.Variables, "var", "v", []string{}, "set a variable (can be set more than once)")
	cmd.Flags().BoolVarP(&options.Wait, "wait", "w", false, "wait for the pipeline to be fully deployed")
	cmd.Flags().DurationVarP(&options.Timeout, "timeout", "t", (5 * time.Minute), "the length of time to wait for completion, zero means never. Any other values should contain a corresponding time unit e.g. 1s, 2m, 3h ")

	return cmd
}

func (dc *deployCommand) runDeploy(ctx context.Context, cwd string, opts *Options) error {
	log.Debugf("creating temporal kubeconfig file '%s'", dc.tempKubeconfigFile)
	if err := dc.kubeconfig.Modify(dc.proxy.GetPort(), dc.proxy.GetToken(), dc.tempKubeconfigFile); err != nil {
		log.Errorf("could not create temporal kubeconfig %s", err)
		return err
	}

	// Read manifest file with the commands to be executed
	manifest, err := dc.getManifest(cwd, opts.Name, opts.ManifestPath)
	if err != nil {
		log.Errorf("could not find manifest file to be executed: %s", err)
		return err
	}

	log.Debugf("starting server on %d", dc.proxy.GetPort())
	dc.proxy.Start()

	defer dc.cleanUp(ctx)

	opts.Variables = append(
		opts.Variables,
		// Set KUBECONFIG environment variable as environment for the commands to be executed
		fmt.Sprintf("KUBECONFIG=%s", dc.tempKubeconfigFile),
		// Set OKTETO_WITHIN_DEPLOY_COMMAND_CONTEXT env variable, so all the Okteto commands executed within this command execution
		// should not overwrite the server and the credentials in the kubeconfig
		"OKTETO_WITHIN_DEPLOY_COMMAND_CONTEXT=true",
	)

	for _, command := range manifest.Deploy {
		if err := dc.executor.Execute(command, opts.Variables); err != nil {
			log.Errorf("error executing command '%s': %s", command, err.Error())
			return err
		}
	}

	if opts.Wait {
		spinner := utils.NewSpinner("Waiting to be fully deployed...")
		spinner.Start()
		defer spinner.Stop()
		if err := dc.waitUntilDeployed(ctx, opts); err != nil {
			return err
		}
		if err := dc.showEndpoints(ctx, opts, spinner); err != nil {
			log.Errorf("could not retrieve endpoints: %s", err)
		}
	}

	return nil
}

func (dc *deployCommand) cleanUp(ctx context.Context) {
	log.Debugf("removing temporal kubeconfig file '%s'", dc.tempKubeconfigFile)
	if err := os.Remove(dc.tempKubeconfigFile); err != nil {
		log.Errorf("could not remove temporal kubeconfig file: %s", err)
	}

	log.Debugf("stopping local server...")
	if err := dc.proxy.Shutdown(ctx); err != nil {
		log.Errorf("could not stop local server: %s", err)
	}
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

func getProxyHandler(name, token string, clusterConfig *rest.Config) (http.Handler, error) {
	// By default we don't disable HTTP/2
	trans, err := newProtocolTransport(clusterConfig, false)
	if err != nil {
		log.Errorf("could not get http transport from config: %s", err)
		return nil, err
	}

	handler := http.NewServeMux()

	destinationURL := &url.URL{
		Host:   strings.TrimPrefix(clusterConfig.Host, "https://"),
		Scheme: "https",
	}
	proxy := httputil.NewSingleHostReverseProxy(destinationURL)
	proxy.Transport = trans

	log.Debugf("forwarding host: %s", clusterConfig.Host)

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
			log.Debugf("detected SPDY request, disabling HTTP/2 for request %s %s", r.Method, r.URL.String())
			// In case of a SPDY request, we create a new proxy with HTTP/2 disabled
			t, err := newProtocolTransport(clusterConfig, true)
			if err != nil {
				log.Errorf("could not disabled HTTP/2: %s", err)
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
				log.Errorf("could not read the request body: %s", err)
				rw.WriteHeader(500)
				return
			}

			if len(b) == 0 {
				reverseProxy.ServeHTTP(rw, r)
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
		reverseProxy.ServeHTTP(rw, r)
	})

	return handler, nil

}

func (dc *deployCommand) waitUntilDeployed(ctx context.Context, opts *Options) error {
	labelSelector := fmt.Sprintf("%s=%s", model.DeployedByLabel, opts.Name)
	c, _, err := okteto.GetK8sClient()
	if err != nil {
		return err
	}

	ticker := time.NewTicker(1 * time.Second)
	to := time.NewTicker(opts.Timeout)
	for {
		select {
		case <-to.C:
			return fmt.Errorf("pipeline '%s' didn't finish after %s", opts.Name, opts.Timeout.String())
		case <-ticker.C:
			isReady := true
			dList, err := deployments.List(ctx, okteto.Context().Namespace, labelSelector, c)
			if err != nil {
				return err
			}
			for _, dep := range dList {
				if dep.Status.ReadyReplicas != dep.Status.Replicas {
					isReady = false
				}
			}
			sfsList, err := statefulsets.List(ctx, okteto.Context().Namespace, labelSelector, c)
			if err != nil {
				return err
			}
			for _, sfs := range sfsList {
				if sfs.Status.ReadyReplicas != sfs.Status.Replicas {
					isReady = false
				}
			}

			if !isReady {
				continue
			}
			return nil
		}
	}
}

func (dc *deployCommand) showEndpoints(ctx context.Context, opts *Options, spinner *utils.Spinner) error {
	spinner.Update("retrieving endpoints")
	pods, err := getDeployPodNames(ctx, opts)
	if err != nil {
		return err
	}

	c, _, err := okteto.GetK8sClient()
	if err != nil {
		return err
	}
	endpoints, err := endpoints.GetEndpointsByPodName(ctx, pods, okteto.Context().Namespace, c)
	if err != nil {
		return err
	}

	svcsName := []string{}
	for _, endpoint := range endpoints {
		svcsName = append(svcsName, endpoint.Name)
	}

	iClient, err := ingresses.GetClient(ctx, c)
	if err != nil {
		return err
	}
	eps, err := iClient.GetEndpointsByEndpointsServices(ctx, okteto.Context().Namespace, svcsName)
	if err != nil {
		return err
	}
	spinner.Stop()
	if len(eps) > 0 {
		sort.Slice(eps, func(i, j int) bool {
			return len(eps[i]) < len(eps[j])
		})
		log.Information("Endpoints available:\n  - %s\n", strings.Join(eps, "\n  - "))
	}
	return nil
}

func getDeployPodNames(ctx context.Context, opts *Options) ([]string, error) {
	uuids := []types.UID{}
	labelSelector := fmt.Sprintf("%s=%s", model.DeployedByLabel, opts.Name)
	c, _, err := okteto.GetK8sClient()
	if err != nil {
		return []string{}, err
	}
	dList, err := deployments.List(ctx, okteto.Context().Namespace, labelSelector, c)
	if err != nil {
		return []string{}, err
	}
	for _, dep := range dList {
		rs, err := replicasets.GetReplicaSetByDeployment(ctx, &dep, c)
		if err != nil {
			return []string{}, err
		}
		uuids = append(uuids, rs.UID)
	}
	sfsList, err := statefulsets.List(ctx, okteto.Context().Namespace, labelSelector, c)
	if err != nil {
		return []string{}, err
	}
	for _, sfs := range sfsList {
		uuids = append(uuids, sfs.UID)
	}
	pList, err := pods.List(ctx, okteto.Context().Namespace, "", c)
	if err != nil {
		return []string{}, err
	}

	podList := []string{}
	for _, p := range pList {
		if isPodFromDeploy(uuids, p.OwnerReferences) {
			podList = append(podList, p.Name)
		}
	}

	return podList, nil
}

func isPodFromDeploy(uuids []types.UID, ownerReferences []metav1.OwnerReference) bool {
	for _, id := range uuids {
		for _, ownerReference := range ownerReferences {
			if id == ownerReference.UID {
				return true
			}
		}

	}
	return false
}
