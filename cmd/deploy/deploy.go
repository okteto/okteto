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

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/namespace"
	"github.com/okteto/okteto/cmd/pipeline"
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
	Manifest     *model.Manifest
	Build        bool

	Repository string
	Branch     string
	Wait       bool
	Timeout    time.Duration
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
	GetManifest func(path string) (*model.Manifest, error)

	Proxy              proxyInterface
	Kubeconfig         kubeConfigHandler
	Executor           utils.ManifestExecutor
	TempKubeconfigFile string
	K8sClientProvider  okteto.K8sClientProvider
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

			if shouldExecuteRemotely(options) {
				remoteOpts := &pipeline.DeployOptions{
					Branch:     options.Branch,
					Repository: options.Repository,
					Name:       options.Name,
					Namespace:  options.Namespace,
					Wait:       options.Wait,
					File:       options.ManifestPath,
					Variables:  options.Variables,
					Timeout:    options.Timeout,
				}
				return pipeline.ExecuteDeployPipeline(ctx, remoteOpts)
			}
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

			kubeconfig := NewKubeConfig()

			proxy, err := NewProxy(options.Name, kubeconfig)
			if err != nil {
				oktetoLog.Infof("could not configure local proxy: %s", err)
				return err
			}

			c := &DeployCommand{
				GetManifest:        model.GetManifestV2,
				Kubeconfig:         kubeconfig,
				Executor:           utils.NewExecutor(oktetoLog.GetOutputFormat()),
				Proxy:              proxy,
				TempKubeconfigFile: GetTempKubeConfigFile(options.Name),
				K8sClientProvider:  okteto.NewK8sClientProvider(),
			}
			return c.RunDeploy(ctx, options)
		},
	}

	cmd.Flags().StringVar(&options.Name, "name", "", "application name")
	cmd.Flags().StringVarP(&options.ManifestPath, "file", "f", "", "path to the okteto manifest file")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "overwrites the namespace where the application is deployed")
	cmd.Flags().StringArrayVarP(&options.Variables, "var", "v", []string{}, "set a variable (can be set more than once)")
	cmd.Flags().BoolVarP(&options.Build, "build", "", false, "force build of images when deploying the app")
	cmd.Flags().MarkHidden("build")

	cmd.Flags().StringVarP(&options.Repository, "repository", "r", "", "the repository to deploy (defaults to the current repository)")
	cmd.Flags().StringVarP(&options.Branch, "branch", "b", "", "the branch to deploy (defaults to the current branch)")
	cmd.Flags().BoolVarP(&options.Wait, "wait", "w", false, "wait until the pipeline finishes (defaults to false)")
	cmd.Flags().DurationVarP(&options.Timeout, "timeout", "t", (5 * time.Minute), "the length of time to wait for completion, zero means never. Any other values should contain a corresponding time unit e.g. 1s, 2m, 3h ")

	return cmd
}

// RunDeploy runs the deploy sequence
func (dc *DeployCommand) RunDeploy(ctx context.Context, deployOptions *Options) error {
	oktetoLog.Debugf("creating temporal kubeconfig file '%s'", dc.TempKubeconfigFile)
	if err := dc.Kubeconfig.Modify(dc.Proxy.GetPort(), dc.Proxy.GetToken(), dc.TempKubeconfigFile); err != nil {
		oktetoLog.Infof("could not create temporal kubeconfig %s", err)
		return err
	}
	var err error
	deployOptions.Manifest, err = dc.GetManifest(deployOptions.ManifestPath)
	if err != nil {
		return err
	}
	oktetoLog.Debug("found okteto manifest")
	if deployOptions.Manifest.Deploy == nil {
		return fmt.Errorf("found okteto manifest, but no deploy commands where defined")
	}
	if deployOptions.Manifest.Context == "" {
		deployOptions.Manifest.Context = okteto.Context().Name
	}
	if deployOptions.Manifest.Namespace == okteto.Context().Namespace {
		deployOptions.Manifest.Namespace = okteto.Context().Namespace
	}

	os.Setenv(model.OktetoNameEnvVar, deployOptions.Name)

	if utils.LoadBoolean(model.OktetoManifestV2Enabled) {

		if deployOptions.Manifest.Build != nil {

			for service, mOptions := range deployOptions.Manifest.Build {
				buildOptions := build.BuildOptions{OutputMode: oktetoLog.GetOutputFormat()}
				if mOptions.Name == "" {
					mOptions.Name = deployOptions.Name
				}
				opts := build.OptsFromManifest(service, mOptions, buildOptions)

				if deployOptions.Build {
					oktetoLog.Debug("force build from manifest definition")
					if err := runBuildAndSetEnvs(ctx, service, opts); err != nil {
						return err
					}
					continue
				}

				if build.ShouldOptimizeBuild(opts.Tag) {
					oktetoLog.Debug("found OKTETO_GIT_COMMIT, optimizing the build flow")
					if skipBuild, err := checkImageAtGlobalAndSetEnvs(service, opts); err != nil {
						return err
					} else if skipBuild {
						continue
					}
				}

				if imageWithDigest, err := registry.GetImageTagWithDigest(opts.Tag); err == oktetoErrors.ErrNotFound {
					oktetoLog.Debugf("image not found, building image %s", opts.Tag)
					if err := runBuildAndSetEnvs(ctx, service, opts); err != nil {
						return err
					}
					continue
				} else if err != nil {
					return fmt.Errorf("error checking image at registry %s: %v", opts.Tag, err)
				} else {
					oktetoLog.Debug("image found, skipping build")
					if err := setManifestEnvVars(service, imageWithDigest); err != nil {
						return err
					}
				}

			}
		}
	}

	deployOptions.Manifest, err = deployOptions.Manifest.ExpandEnvVars()
	if err != nil {
		return err
	}

	oktetoLog.Debugf("starting server on %d", dc.Proxy.GetPort())
	dc.Proxy.Start()

	defer dc.cleanUp(ctx)

	for _, variable := range deployOptions.Variables {
		value := strings.SplitN(variable, "=", 2)[1]
		if strings.TrimSpace(value) != "" {
			oktetoLog.AddMaskedWord(value)
		}
	}
	deployOptions.Variables = append(
		deployOptions.Variables,
		// Set KUBECONFIG environment variable as environment for the commands to be executed
		fmt.Sprintf("%s=%s", model.KubeConfigEnvVar, dc.TempKubeconfigFile),
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
	oktetoLog.EnableMasking()

	for _, command := range deployOptions.Manifest.Deploy.Commands {
		oktetoLog.SetStage(command.Name)
		if err := dc.Executor.Execute(command, deployOptions.Variables); err != nil {
			oktetoLog.Infof("error executing command '%s': %s", command.Name, err.Error())
			return fmt.Errorf("error executing command '%s': %s", command.Name, err.Error())
		}
	}
	oktetoLog.SetStage("")
	oktetoLog.DisableMasking()

	if !utils.LoadBoolean(model.OktetoWithinDeployCommandContextEnvVar) {
		if err := dc.showEndpoints(ctx, deployOptions); err != nil {
			oktetoLog.Infof("could not retrieve endpoints: %s", err)
		}
	}

	return nil
}

func checkImageAtGlobalAndSetEnvs(service string, options build.BuildOptions) (bool, error) {
	globalReference := strings.Replace(options.Tag, okteto.DevRegistry, okteto.GlobalRegistry, 1)

	imageWithDigest, err := registry.GetImageTagWithDigest(globalReference)
	if err == oktetoErrors.ErrNotFound {
		oktetoLog.Debug("image not found at global registry, not running optimization for deployment")
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if err := setManifestEnvVars(service, imageWithDigest); err != nil {
		return false, err
	}
	oktetoLog.Debug("image found at global registry, running optimization for deployment")
	return true, nil

}

func runBuildAndSetEnvs(ctx context.Context, service string, options build.BuildOptions) error {
	oktetoLog.Information("building image for service %s", service)
	if err := build.Run(ctx, options); err != nil {
		return err
	}
	imageWithDigest, err := registry.GetImageTagWithDigest(options.Tag)
	if err != nil {
		return fmt.Errorf("error checking image at registry %s: %v", options.Tag, err)
	}
	return setManifestEnvVars(service, imageWithDigest)
}

func setManifestEnvVars(service, reference string) error {
	reg, repo, tag, image := registry.GetReferecenceEnvs(reference)

	oktetoLog.Debugf("envs registry=%s repository=%s image=%s tag=%s", reg, repo, image, tag)

	os.Setenv(fmt.Sprintf("OKTETO_BUILD_%s_REGISTRY", strings.ToUpper(service)), reg)
	os.Setenv(fmt.Sprintf("OKTETO_BUILD_%s_REPOSITORY", strings.ToUpper(service)), repo)
	os.Setenv(fmt.Sprintf("OKTETO_BUILD_%s_IMAGE", strings.ToUpper(service)), reference)
	os.Setenv(fmt.Sprintf("OKTETO_BUILD_%s_TAG", strings.ToUpper(service)), tag)

	oktetoLog.Debug("manifest env vars set")
	return nil
}

func (dc *DeployCommand) cleanUp(ctx context.Context) {
	oktetoLog.Debugf("removing temporal kubeconfig file '%s'", dc.TempKubeconfigFile)
	if err := os.Remove(dc.TempKubeconfigFile); err != nil {
		oktetoLog.Infof("could not remove temporal kubeconfig file: %s", err)
	}

	oktetoLog.Debugf("stopping local server...")
	if err := dc.Proxy.Shutdown(ctx); err != nil {
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

//GetTempKubeConfigFile returns a where the temp kubeConfigFile should be stored
func GetTempKubeConfigFile(name string) string {
	return fmt.Sprintf(tempKubeConfigTemplate, config.GetUserHomeDir(), name)
}

func (dc *DeployCommand) showEndpoints(ctx context.Context, opts *Options) error {
	spinner := utils.NewSpinner("Retrieving endpoints...")
	spinner.Start()
	defer spinner.Stop()
	labelSelector := fmt.Sprintf("%s=%s", model.DeployedByLabel, opts.Name)
	iClient, err := dc.K8sClientProvider.GetIngressClient(ctx)
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
		oktetoLog.Information("Endpoints available:\n  - %s\n", strings.Join(eps, "\n  - "))

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
		if !isClean {
			sha = utils.GetRandomSHA(ctx, cwd)
		}
		value := fmt.Sprintf("%s%s", model.OktetoGitCommitPrefix, sha)
		os.Setenv(model.OktetoGitCommitEnvVar, value)
	}
	if os.Getenv(model.OktetoRegistryURLEnvVar) == "" {
		os.Setenv(model.OktetoRegistryURLEnvVar, okteto.Context().Registry)
	}
	if os.Getenv(model.OktetoBuildkitHostURLEnvVar) == "" {
		os.Setenv(model.OktetoBuildkitHostURLEnvVar, okteto.Context().Builder)
	}
	return nil
}

func shouldExecuteRemotely(options *Options) bool {
	return options.Branch != "" || options.Repository != ""
}
