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
	"github.com/okteto/okteto/cmd/namespace"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
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
	Variables    []string
	Timeout      time.Duration
	Manifest     *model.Manifest
	Build        bool
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
	getManifest func(cwd string, opts contextCMD.ManifestOptions) (*model.Manifest, error)

	proxy              proxyInterface
	kubeconfig         kubeConfigHandler
	executor           utils.ManifestExecutor
	tempKubeconfigFile string
	k8sClientProvider  okteto.K8sClientProvider
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

			if err := contextCMD.LoadManifestV2WithContext(ctx, options.Namespace, options.ManifestPath); err != nil {
				return err
			}

			if okteto.IsOkteto() {
				create, err := utils.ShouldCreateNamespace(ctx, okteto.Context().Namespace)
				if err != nil {
					return err
				}
				if create {
					nsCmd, err := namespace.NewCommand()
					if err != nil {
						return err
					}
					nsCmd.Create(ctx, &namespace.CreateOptions{Namespace: okteto.Context().Namespace})
				}
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get the current working directory: %w", err)
			}

			addEnvVars(ctx, cwd)
			if options.Name == "" {
				options.Name = utils.InferApplicationName(cwd)
			}

			// Look for a free local port to start the proxy
			port, err := model.GetAvailablePort("localhost")
			if err != nil {
				oktetoLog.Infof("could not find a free port to start proxy server: %s", err)
				return err
			}
			oktetoLog.Debugf("found available port %d", port)

			// TODO for now, using self-signed certificates
			cert, err := tls.X509KeyPair(cert, key)
			if err != nil {
				oktetoLog.Infof("could not read certificate: %s", err)
				return err
			}

			// Generate a token for the requests done to the proxy
			sessionToken := uuid.NewString()

			kubeconfig := newKubeConfig()
			clusterConfig, err := kubeconfig.Read()
			if err != nil {
				oktetoLog.Infof("could not read kubeconfig file: %s", err)
				return err
			}

			handler, err := getProxyHandler(options.Name, sessionToken, clusterConfig)
			if err != nil {
				oktetoLog.Infof("could not configure local proxy: %s", err)
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
				getManifest: contextCMD.GetManifest,

				kubeconfig: kubeconfig,
				executor:   utils.NewExecutor(oktetoLog.GetOutputFormat()),
				proxy: newProxy(proxyConfig{
					port:  port,
					token: sessionToken,
				}, s),
				tempKubeconfigFile: fmt.Sprintf(tempKubeConfigTemplate, config.GetUserHomeDir(), options.Name),
				k8sClientProvider:  okteto.NewK8sClientProvider(),
			}
			return c.runDeploy(ctx, cwd, options)
		},
	}

	cmd.Flags().StringVar(&options.Name, "name", "", "application name")
	cmd.Flags().StringVarP(&options.ManifestPath, "file", "f", "", "path to the okteto manifest file")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "overwrites the namespace where the application is deployed")

	cmd.Flags().StringArrayVarP(&options.Variables, "var", "v", []string{}, "set a variable (can be set more than once)")
	cmd.Flags().BoolVarP(&options.Build, "build", "", false, "force build of images when deploying the app")
	cmd.Flags().MarkHidden("build")

	return cmd
}

func (dc *deployCommand) runDeploy(ctx context.Context, cwd string, opts *Options) error {
	oktetoLog.Debugf("creating temporal kubeconfig file '%s'", dc.tempKubeconfigFile)
	if err := dc.kubeconfig.Modify(dc.proxy.GetPort(), dc.proxy.GetToken(), dc.tempKubeconfigFile); err != nil {
		oktetoLog.Infof("could not create temporal kubeconfig %s", err)
		return err
	}
	var err error
	if contextCMD.IsManifestV2Enabled() {
		opts.Manifest, err = contextCMD.GetManifestV2(cwd, opts.ManifestPath)
		if err != nil {
			return err
		}
		oktetoLog.Debug("found okteto manifest")

		if opts.Manifest.Deploy == nil {
			return fmt.Errorf("found okteto manifest, but no deploy commands where defined")
		}

		if opts.Manifest.Build != nil {

			var buildErrs []string

			for service, buildInfo := range opts.Manifest.Build {
				if okteto.Context().IsOkteto && buildInfo.Image == "" {
					buildInfo.Image = fmt.Sprintf("%s/%s-%s:%s", okteto.DevRegistry, opts.Name, service, "okteto")
				}

				buildOptions := build.BuildOptions{OutputMode: oktetoLog.GetOutputFormat()}
				manifestOptions := build.OptsFromManifest(service, buildInfo, buildOptions)

				// if Build flag is disabled, we check if the image is already at the registry and build it prior to deploy
				if !opts.Build {
					if err := buildIfImageNotFound(ctx, manifestOptions); err != nil {
						buildErrs = append(buildErrs, err.Error())
						continue
					}
				} else {
					// if Build flag is enabled, we re-build the image regardless if it is already or not
					oktetoLog.Debug("force build from manifest definition")
					if err := build.Run(ctx, manifestOptions); err != nil {
						buildErrs = append(buildErrs, err.Error())
						continue
					}
				}

				// check that the image is at the registry correctly pushed
				imageWithDigest, err := registry.GetImageTagWithDigest(manifestOptions.Tag)
				if err != nil {
					buildErrs = append(buildErrs, err.Error())
				}
				oktetoLog.Debugf("got digest from registry: %s", imageWithDigest)
				setManifestEnvVars(service, imageWithDigest)
			}
			if len(buildErrs) != 0 {
				return fmt.Errorf("build failed for the services defined at manifest: %v", buildErrs)
			}
		}

		var parsedCommands []string
		for _, command := range opts.Manifest.Deploy.Commands {
			parsedCommands = append(parsedCommands, expandManifestEnvVars(command))
		}
		opts.Manifest.Deploy.Commands = parsedCommands

	} else {
		// Read manifest file with the commands to be executed
		opts.Manifest, err = dc.getManifest(cwd, contextCMD.ManifestOptions{Name: opts.Name, Filename: opts.ManifestPath})
		if err != nil {
			oktetoLog.Infof("could not find manifest file to be executed: %s", err)
			return err
		}

		if opts.Manifest.Deploy == nil {
			return fmt.Errorf("found okteto manifest, but no deploy commands where defined")
		}
	}
	opts.Manifest.Context = okteto.Context().Name
	opts.Manifest.Namespace = okteto.Context().Namespace

	oktetoLog.Debugf("starting server on %d", dc.proxy.GetPort())
	dc.proxy.Start()

	defer dc.cleanUp(ctx)

	opts.Variables = append(
		opts.Variables,
		// Set KUBECONFIG environment variable as environment for the commands to be executed
		fmt.Sprintf("%s=%s", model.KubeConfigEnvVar, dc.tempKubeconfigFile),
		// Set OKTETO_WITHIN_DEPLOY_COMMAND_CONTEXT env variable, so all the Okteto commands executed within this command execution
		// should not overwrite the server and the credentials in the kubeconfig
		fmt.Sprintf("%s=true", model.OktetoWithinDeployCommandContextEnvVar),
		// Set OKTETO_DISABLE_SPINNER=true env variable, so all the Okteto commands disable spinner which leads to errors
		fmt.Sprintf("%s=true", model.OktetoDisableSpinnerEnvVar),
		// Set BUILDKIT_PROGRESS=plain env variable, so all the commands disable docker tty builds
		fmt.Sprintf("%s=true", model.BuildkitProgressEnvVar),
		// Set OKTETO_NAMESPACE=namespace-name env variable, so all the commandsruns on the same namespace
		fmt.Sprintf("%s=%s", model.OktetoNamespaceEnvVar, okteto.Context().Namespace),
	)

	for _, command := range opts.Manifest.Deploy.Commands {
		oktetoLog.SetStage(command)
		if err := dc.executor.Execute(command, opts.Variables); err != nil {
			oktetoLog.Infof("error executing command '%s': %s", command, err.Error())
			return fmt.Errorf("error executing command '%s': %s", command, err.Error())
		}
	}
	oktetoLog.SetStage("")

	if !utils.LoadBoolean(model.OktetoWithinDeployCommandContextEnvVar) {
		if err := dc.showEndpoints(ctx, opts); err != nil {
			oktetoLog.Infof("could not retrieve endpoints: %s", err)
		}
	}

	return nil
}

func setManifestEnvVars(service, reference string) {
	reg, image := registry.GetRegistryAndRepo(reference)
	repository, tag := registry.GetRepoNameAndTag(image)

	oktetoLog.Debugf("envs registry=%s repository=%s image=%s tag=%s", reg, repository, reference, tag)

	os.Setenv(fmt.Sprintf("build.%s.registry", service), reg)
	os.Setenv(fmt.Sprintf("build.%s.repository", service), repository)
	os.Setenv(fmt.Sprintf("build.%s.image", service), reference)
	os.Setenv(fmt.Sprintf("build.%s.tag", service), tag)

	oktetoLog.Debug("manifest env vars set")
}

func expandManifestEnvVars(manifest string) string {
	return os.ExpandEnv(manifest)
}

func buildIfImageNotFound(ctx context.Context, options build.BuildOptions) error {
	_, err := registry.GetImageTagWithDigest(options.Tag)
	if err != nil {
		if err == oktetoErrors.ErrNotFound {
			oktetoLog.Debugf("image not found, building image %s", options.Tag)
			if err := build.Run(ctx, options); err != nil {
				return err
			}
			oktetoLog.Debugf("success building image before deploy %s", options.Tag)
			return nil
		}
		return fmt.Errorf("error calling registry: %s", err.Error())
	}
	oktetoLog.Debug("image found, skipping build")
	return nil
}

func (dc *deployCommand) cleanUp(ctx context.Context) {
	oktetoLog.Debugf("removing temporal kubeconfig file '%s'", dc.tempKubeconfigFile)
	if err := os.Remove(dc.tempKubeconfigFile); err != nil {
		oktetoLog.Infof("could not remove temporal kubeconfig file: %s", err)
	}

	oktetoLog.Debugf("stopping local server...")
	if err := dc.proxy.Shutdown(ctx); err != nil {
		oktetoLog.Infof("could not stop local server: %s", err)
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

			var body map[string]json.RawMessage
			if err := json.Unmarshal(b, &body); err != nil {
				oktetoLog.Infof("could not unmarshal request: %s", err)
				rw.WriteHeader(500)
				return
			}

			m, ok := body["metadata"]
			if !ok {
				oktetoLog.Info("request body doesn't have metadata field")
				rw.WriteHeader(500)
				return
			}

			var metadata metav1.ObjectMeta
			if err := json.Unmarshal(m, &metadata); err != nil {
				oktetoLog.Infof("could not process resource's metadata: %s", err)
				rw.WriteHeader(500)
				return
			}

			if metadata.Labels == nil {
				metadata.Labels = map[string]string{}
			}
			metadata.Labels[model.DeployedByLabel] = name

			if metadata.Annotations == nil {
				metadata.Annotations = map[string]string{}
			}
			if utils.IsOktetoRepo() {
				metadata.Annotations[model.OktetoSampleAnnotation] = "true"
			}

			metadataAsByte, err := json.Marshal(metadata)
			if err != nil {
				oktetoLog.Infof("could not process resource's metadata: %s", err)
				rw.WriteHeader(500)
				return
			}

			body["metadata"] = metadataAsByte

			b, err = json.Marshal(body)
			if err != nil {
				oktetoLog.Infof("could not marshal modified body: %s", err)
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

func (dc *deployCommand) showEndpoints(ctx context.Context, opts *Options) error {
	spinner := utils.NewSpinner("Retrieving endpoints...")
	spinner.Start()
	defer spinner.Stop()
	labelSelector := fmt.Sprintf("%s=%s", model.DeployedByLabel, opts.Name)
	iClient, err := dc.k8sClientProvider.GetIngressClient(ctx)
	if err != nil {
		return err
	}
	eps, err := iClient.GetEndpointsBySelector(ctx, okteto.Context().Namespace, labelSelector)
	if err != nil {
		return err
	}
	spinner.Stop()
	if len(eps) > 0 {
		sort.Slice(eps, func(i, j int) bool {
			return len(eps[i]) < len(eps[j])
		})
		switch oktetoLog.GetOutputFormat() {
		case oktetoLog.TTYFormat:
			oktetoLog.Printf("Endpoints available: %s\n", eps)
		default:
			oktetoLog.Information("Endpoints available:\n  - %s\n", strings.Join(eps, "\n  - "))
		}

	}
	return nil
}

func addEnvVars(ctx context.Context, cwd string) error {
	if os.Getenv(model.OktetoGitBranchEnvVar) == "" {
		branch, err := utils.GetBranch(ctx, cwd)
		if err != nil {
			oktetoLog.Infof("could not retrieve branch name: %s", err)
		}
		os.Setenv(model.OktetoGitBranchEnvVar, branch)
	}

	if os.Getenv(model.OktetoGitCommitEnvVar) == "" {
		sha, err := utils.GetGitCommit(ctx, cwd)
		if err != nil {
			oktetoLog.Infof("could not retrieve sha: %s", err)
		}
		isClean, err := utils.IsCleanDirectory(ctx, cwd)
		if err != nil {
			oktetoLog.Infof("could not status: %s", err)
		}
		if isClean {
			os.Setenv(model.OktetoGitCommitEnvVar, sha)
		} else {
			sha := utils.GetRandomSHA(ctx, cwd)
			os.Setenv(model.OktetoGitCommitEnvVar, sha)
		}
	}
	if os.Getenv(model.OktetoRegistryURLEnvVar) == "" {
		os.Setenv(model.OktetoRegistryURLEnvVar, okteto.Context().Registry)
	}
	if os.Getenv(model.OktetoBuildkitHostURLEnvVar) == "" {
		os.Setenv(model.OktetoBuildkitHostURLEnvVar, okteto.Context().Builder)
	}
	return nil
}
