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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

const headerUpgrade = "Upgrade"

var tempKubeConfigTemplate = "%s/.okteto/kubeconfig-%s"

// Options options for deploy command
type Options struct {
	ManifestPath string
	Name         string
	Namespace    string
	K8sContext   string
	Variables    []string
	Manifest     *model.Manifest
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

//DeployCommand defines the config for deploying an app
type DeployCommand struct {
	GetManifest func(ctx context.Context, srcFolder string, opts contextCMD.ManifestOptions) (*model.Manifest, error)

	Proxy              proxyInterface
	Kubeconfig         kubeConfigHandler
	Executor           utils.ManifestExecutor
	TempKubeconfigFile string
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
			os.Setenv(model.OktetoWithinDeployCommandContextEnvVar, "false")
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

			kubeconfig := NewKubeConfig()

			proxy, err := NewProxy(options.Name, kubeconfig)
			if err != nil {
				log.Infof("could not configure local proxy: %s", err)
				return err
			}

			c := &DeployCommand{
				GetManifest: contextCMD.GetManifest,

				Kubeconfig:         kubeconfig,
				Executor:           utils.NewExecutor(),
				Proxy:              proxy,
				TempKubeconfigFile: GetTempKubeConfigFile(options.Name),
			}
			return c.RunDeploy(ctx, cwd, options)
		},
	}

	cmd.Flags().StringVar(&options.Name, "name", "", "application name")
	cmd.Flags().StringVarP(&options.ManifestPath, "file", "f", "", "path to the manifest file")
	cmd.Flags().StringVar(&options.Namespace, "namespace", "", "application name")
	cmd.Flags().StringVar(&options.K8sContext, "context", "", "k8s context")

	cmd.Flags().StringArrayVarP(&options.Variables, "var", "v", []string{}, "set a variable (can be set more than once)")

	return cmd
}

//RunDeploy runs the sequence to deploy an application on a cluster
func (dc *DeployCommand) RunDeploy(ctx context.Context, cwd string, opts *Options) error {
	log.Debugf("creating temporal kubeconfig file '%s'", dc.TempKubeconfigFile)
	if err := dc.Kubeconfig.Modify(dc.Proxy.GetPort(), dc.Proxy.GetToken(), dc.TempKubeconfigFile); err != nil {
		log.Infof("could not create temporal kubeconfig %s", err)
		return err
	}

	var err error
	opts.Manifest, err = dc.GetManifest(ctx, cwd, contextCMD.ManifestOptions{Name: opts.Name, Filename: opts.ManifestPath})
	if err != nil {
		log.Infof("could not find manifest file to be executed: %s", err)
		return err
	}

	log.Debugf("starting server on %d", dc.Proxy.GetPort())
	dc.Proxy.Start()

	defer dc.cleanUp(ctx)

	opts.Variables = append(
		opts.Variables,
		// Set KUBECONFIG environment variable as environment for the commands to be executed
		fmt.Sprintf("KUBECONFIG=%s", dc.TempKubeconfigFile),
		// Set OKTETO_WITHIN_DEPLOY_COMMAND_CONTEXT env variable, so all the Okteto commands executed within this command execution
		// should not overwrite the server and the credentials in the kubeconfig
		"OKTETO_WITHIN_DEPLOY_COMMAND_CONTEXT=true",
	)

	for _, command := range opts.Manifest.Deploy.Commands {
		if err := dc.Executor.Execute(command, opts.Variables); err != nil {
			log.Infof("error executing command '%s': %s", command, err.Error())
			return err
		}
	}

	return nil
}

func (dc *DeployCommand) cleanUp(ctx context.Context) {
	log.Debugf("removing temporal kubeconfig file '%s'", dc.TempKubeconfigFile)
	if err := os.Remove(dc.TempKubeconfigFile); err != nil {
		log.Infof("could not remove temporal kubeconfig file: %s", err)
	}

	log.Debugf("stopping local server...")
	if err := dc.Proxy.Shutdown(ctx); err != nil {
		log.Infof("could not stop local server: %s", err)
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
		log.Infof("could not get http transport from config: %s", err)
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
				log.Infof("could not disabled HTTP/2: %s", err)
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
				log.Infof("could not read the request body: %s", err)
				rw.WriteHeader(500)
				return
			}
			defer r.Body.Close()

			defer r.Body.Close()
			if len(b) == 0 {
				reverseProxy.ServeHTTP(rw, r)
				return
			}

			var body map[string]json.RawMessage
			if err := json.Unmarshal(b, &body); err != nil {
				log.Infof("could not unmarshal request: %s", err)
				rw.WriteHeader(500)
				return
			}

			m, ok := body["metadata"]
			if !ok {
				log.Info("request body doesn't have metadata field")
				rw.WriteHeader(500)
				return
			}

			var metadata metav1.ObjectMeta
			if err := json.Unmarshal(m, &metadata); err != nil {
				log.Infof("could not process resource's metadata: %s", err)
				rw.WriteHeader(500)
				return
			}

			if metadata.Labels == nil {
				metadata.Labels = map[string]string{}
			}
			metadata.Labels[model.DeployedByLabel] = name

			metadataAsByte, err := json.Marshal(metadata)
			if err != nil {
				log.Infof("could not process resource's metadata: %s", err)
				rw.WriteHeader(500)
				return
			}

			body["metadata"] = metadataAsByte

			b, err = json.Marshal(body)
			if err != nil {
				log.Infof("could not marshal modified body: %s", err)
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

//GetTempKubeConfigFile returns a where the temp kubeConfigFile should be stored
func GetTempKubeConfigFile(name string) string {
	return fmt.Sprintf(tempKubeConfigTemplate, config.GetUserHomeDir(), name)
}
