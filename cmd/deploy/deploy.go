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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strings"
	"time"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/namespace"
	pipelineCMD "github.com/okteto/okteto/cmd/pipeline"
	stackCMD "github.com/okteto/okteto/cmd/stack"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/cmd/utils/displayer"
	"github.com/okteto/okteto/cmd/utils/executor"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/cmd/stack"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

const (
	headerUpgrade          = "Upgrade"
	succesfullyDeployedmsg = "Development environment '%s' successfully deployed"
)

var tempKubeConfigTemplate = "%s/.okteto/kubeconfig-%s"

// Options options for deploy command
type Options struct {
	ManifestPath string
	Name         string
	Namespace    string
	Variables    []string
	Manifest     *model.Manifest
	Build        bool
	Dependencies bool

	Repository string
	Branch     string
	Wait       bool
	Timeout    time.Duration

	ShowCTA bool
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
	Executor           executor.ManifestExecutor
	TempKubeconfigFile string
	K8sClientProvider  okteto.K8sClientProvider

	PipelineType model.Archetype
}

//Deploy deploys the okteto manifest
func Deploy(ctx context.Context) *cobra.Command {
	options := &Options{}

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Execute the list of commands specified in the 'deploy' section of your okteto manifest",
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#deploy"),
		RunE: func(cmd *cobra.Command, args []string) error {

			if shouldExecuteRemotely(options) {
				remoteOpts := &pipelineCMD.DeployOptions{
					Branch:     options.Branch,
					Repository: options.Repository,
					Name:       options.Name,
					Namespace:  options.Namespace,
					Wait:       options.Wait,
					File:       options.ManifestPath,
					Variables:  options.Variables,
					Timeout:    options.Timeout,
				}
				return pipelineCMD.ExecuteDeployPipeline(ctx, remoteOpts)
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
				options.Name = utils.InferName(cwd)
			}
			options.ShowCTA = oktetoLog.IsInteractive()

			kubeconfig := NewKubeConfig()

			proxy, err := NewProxy(options.Name, kubeconfig)
			if err != nil {
				oktetoLog.Infof("could not configure local proxy: %s", err)
				return err
			}

			c := &DeployCommand{
				GetManifest:        model.GetManifestV2,
				Kubeconfig:         kubeconfig,
				Executor:           executor.NewExecutor(oktetoLog.GetOutputFormat()),
				Proxy:              proxy,
				TempKubeconfigFile: GetTempKubeConfigFile(options.Name),
				K8sClientProvider:  okteto.NewK8sClientProvider(),
			}
			startTime := time.Now()
			err = c.RunDeploy(ctx, options)
			duration := time.Since(startTime)
			analytics.TrackDeploy(err == nil, utils.IsOktetoRepo(), err, duration, c.PipelineType)
			return err
		},
	}

	cmd.Flags().StringVar(&options.Name, "name", "", "development environment name")
	cmd.Flags().StringVarP(&options.ManifestPath, "file", "f", "", "path to the okteto manifest file")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "overwrites the namespace where the development environment is deployed")
	cmd.Flags().StringArrayVarP(&options.Variables, "var", "v", []string{}, "set a variable (can be set more than once)")
	cmd.Flags().BoolVarP(&options.Build, "build", "", false, "force build of images when deploying the development environment")
	cmd.Flags().BoolVarP(&options.Dependencies, "dependencies", "", false, "deploy the dependencies from manifest")

	cmd.Flags().StringVarP(&options.Repository, "repository", "r", "", "the repository to deploy (defaults to the current repository)")
	cmd.Flags().StringVarP(&options.Branch, "branch", "b", "", "the branch to deploy (defaults to the current branch)")
	cmd.Flags().BoolVarP(&options.Wait, "wait", "w", false, "wait until the development environment is deployed (defaults to false)")
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
		return oktetoErrors.ErrManifestFoundButNoDeployCommands
	}
	if deployOptions.Manifest.Context == "" {
		deployOptions.Manifest.Context = okteto.Context().Name
	}
	if deployOptions.Manifest.Namespace == "" {
		deployOptions.Manifest.Namespace = okteto.Context().Namespace
	}
	dc.PipelineType = deployOptions.Manifest.Type

	os.Setenv(model.OktetoNameEnvVar, deployOptions.Name)

	if deployOptions.Dependencies && !okteto.IsOkteto() {
		return fmt.Errorf("'dependencies' is only available in clusters managed by Okteto")
	}

	if deployOptions.Build {
		for service, mBuildInfo := range deployOptions.Manifest.Build {
			oktetoLog.Debug("force build from manifest definition")
			if err := runBuildAndSetEnvs(ctx, service, mBuildInfo); err != nil {
				return err
			}
		}

	} else if err := checkBuildFromManifest(ctx, deployOptions.Manifest.Build); err != nil {
		return err
	}

	for depName, dep := range deployOptions.Manifest.Dependencies {
		pipOpts := &pipelineCMD.DeployOptions{
			Name:         depName,
			Repository:   dep.Repository,
			Branch:       dep.Branch,
			File:         dep.ManifestPath,
			Variables:    model.SerializeBuildArgs(dep.Variables),
			Wait:         dep.Wait,
			Timeout:      deployOptions.Timeout,
			SkipIfExists: !deployOptions.Dependencies,
		}
		if err := pipelineCMD.ExecuteDeployPipeline(ctx, pipOpts); err != nil {
			return err
		}
	}

	deployOptions.Manifest, err = deployOptions.Manifest.ExpandEnvVars()
	if err != nil {
		return err
	}

	oktetoLog.Debugf("starting server on %d", dc.Proxy.GetPort())
	dc.Proxy.Start()

	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Deploying '%s'...", deployOptions.Name)
	data := &pipeline.CfgData{
		Name:       deployOptions.Name,
		Namespace:  deployOptions.Manifest.Namespace,
		Repository: os.Getenv(model.GithubRepositoryEnvVar),
		Branch:     os.Getenv(model.OktetoGitBranchEnvVar),
		Filename:   deployOptions.Manifest.Filename,
		Status:     pipeline.ProgressingStatus,
		Manifest:   deployOptions.Manifest.Manifest,
		Icon:       deployOptions.Manifest.Icon,
	}

	if !deployOptions.Manifest.IsV2 && deployOptions.Manifest.Type == model.StackType {
		data.Manifest = deployOptions.Manifest.Deploy.Compose.Stack.Manifest
	}
	c, _, err := dc.K8sClientProvider.Provide(okteto.Context().Cfg)
	if err != nil {
		return err
	}
	cfg, err := pipeline.TranslateConfigMapAndDeploy(ctx, data, c)
	if err != nil {
		return err
	}

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
		// Set OKTETO_NAMESPACE=namespace-name env variable, so all the commandsruns on the same namespace
		fmt.Sprintf("%s=%s", model.OktetoNamespaceEnvVar, okteto.Context().Namespace),
	)
	oktetoLog.EnableMasking()

	err = dc.deploy(ctx, deployOptions)

	oktetoLog.SetStage("")
	oktetoLog.DisableMasking()

	if err != nil {
		err = oktetoErrors.UserError{
			E:    oktetoErrors.ErrDeployHasFailedCommand,
			Hint: "Update the 'deploy' section of your okteto manifest and try again",
		}
		oktetoLog.AddToBuffer(oktetoLog.InfoLevel, err.Error())
		data.Status = pipeline.ErrorStatus
	} else {
		hasDeployed, err := pipeline.HasDeployedSomething(ctx, deployOptions.Name, deployOptions.Manifest.Namespace, c)
		if err != nil {
			return err
		}
		if hasDeployed {
			if deployOptions.Wait {
				if err := dc.wait(ctx, deployOptions); err != nil {
					return err
				}
			}
			if !utils.LoadBoolean(model.OktetoWithinDeployCommandContextEnvVar) {
				if err := dc.showEndpoints(ctx, deployOptions); err != nil {
					oktetoLog.Infof("could not retrieve endpoints: %s", err)
				}
			}
			if deployOptions.ShowCTA {
				oktetoLog.Success(succesfullyDeployedmsg, deployOptions.Name)
				if oktetoLog.IsInteractive() {
					oktetoLog.Information("Run 'okteto up' to activate your development container")
				}
			}
			data.Status = pipeline.DeployedStatus
		} else {
			err = oktetoErrors.UserError{
				E:    oktetoErrors.ErrDeployHasNotDeployAnyResource,
				Hint: "Update the 'deploy' section of your manifest and try again",
			}
			oktetoLog.AddToBuffer(oktetoLog.InfoLevel, err.Error())
			data.Status = pipeline.ErrorStatus
		}
	}

	if err := pipeline.UpdateConfigMap(ctx, cfg, data, c); err != nil {
		return err
	}
	return err
}

func (dc *DeployCommand) deploy(ctx context.Context, opts *Options) error {
	stopCmds := make(chan os.Signal, 1)
	signal.Notify(stopCmds, os.Interrupt)
	exit := make(chan error, 1)
	go func() {
		for _, command := range opts.Manifest.Deploy.Commands {
			oktetoLog.SetStage(command.Name)
			if err := dc.Executor.Execute(command, opts.Variables); err != nil {
				oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, "error executing command '%s': %s", command.Name, err.Error())
				exit <- fmt.Errorf("error executing command '%s': %s", command.Name, err.Error())
				return
			}
		}
		exit <- nil
	}()
	select {
	case <-stopCmds:
		oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
		sp := utils.NewSpinner("Shutting down...")
		sp.Start()
		defer sp.Stop()
		dc.Executor.CleanUp(errors.New("interrupt signal received"))
		return oktetoErrors.ErrIntSig
	case err := <-exit:
		if err != nil {
			return err
		}
	}

	stopCompose := make(chan os.Signal, 1)
	signal.Notify(stopCompose, os.Interrupt)
	exitCompose := make(chan error, 1)

	go func() {
		if opts.Manifest.Deploy.Compose != nil {
			oktetoLog.SetStage("Deploying compose")
			reader, writer := io.Pipe()
			oktetoLog.SetOutput(writer)
			exit := make(chan error, 1)
			stop := make(chan os.Signal, 1)
			signal.Notify(stop, os.Interrupt)
			d := displayer.NewDisplayer(oktetoLog.GetOutputFormat(), reader, nil)
			d.Display("Deploying compose")
			go func() {
				err := dc.deployStack(ctx, opts)
				writer.Close()
				exit <- err
			}()
			select {
			case err := <-exit:
				d.CleanUp(err)
			case <-stop:
				d.CleanUp(errors.New("Interrupt signal received"))
				exitCompose <- oktetoErrors.ErrIntSig
			}
			oktetoLog.SetOutput(os.Stdout)
		}
		exitCompose <- nil
	}()

	select {
	case <-stopCmds:
		os.Unsetenv(model.OktetoDisableSpinnerEnvVar)
		oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
		sp := utils.NewSpinner("Shutting down...")
		sp.Start()
		defer sp.Stop()
		dc.Executor.CleanUp(errors.New("Interrupt signal received"))
		return oktetoErrors.ErrIntSig
	case err := <-exitCompose:
		if err != nil {
			return err
		}
	}

	oktetoLog.SetStage("done")
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "EOF")
	oktetoLog.SetStage("")

	return nil
}

func (dc *DeployCommand) deployStack(ctx context.Context, opts *Options) error {
	composeInfo := opts.Manifest.Deploy.Compose
	composeInfo.Stack.Namespace = okteto.Context().Namespace
	stackOpts := &stack.StackDeployOptions{
		StackPaths: composeInfo.Manifest,
		ForceBuild: false,
		Wait:       opts.Wait,
		Timeout:    opts.Timeout,
	}

	c, _, err := dc.K8sClientProvider.Provide(kubeconfig.Get([]string{dc.TempKubeconfigFile}))
	if err != nil {
		return err
	}
	cfg, err := dc.Kubeconfig.Read()
	if err != nil {
		return err
	}
	stackCommand := stackCMD.DeployCommand{
		K8sClient:      c,
		Config:         cfg,
		IsInsideDeploy: true,
	}
	os.Setenv(model.OktetoDisableSpinnerEnvVar, "true")

	err = stackCommand.RunDeploy(ctx, composeInfo.Stack, stackOpts)
	if err != nil {
		return err
	}
	os.Unsetenv(model.OktetoDisableSpinnerEnvVar)
	return err
}

func checkImageAtGlobalAndSetEnvs(service string, options build.BuildOptions) (bool, error) {
	globalReference := strings.Replace(options.Tag, okteto.DevRegistry, okteto.GlobalRegistry, 1)

	imageWithDigest, err := registry.GetImageTagWithDigest(globalReference)
	if err == oktetoErrors.ErrNotFound {
		oktetoLog.Debug("image not built at global registry, not running optimization for deployment")
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if err := setManifestEnvVars(service, imageWithDigest); err != nil {
		return false, err
	}
	oktetoLog.Debug("image already built at global registry, running optimization for deployment")
	return true, nil

}

func runBuildAndSetEnvs(ctx context.Context, service string, buildInfo *model.BuildInfo) error {
	oktetoLog.Information("Building image for service '%s'", service)

	options := build.OptsFromManifest(service, buildInfo, build.BuildOptions{})
	if err := build.Run(ctx, options); err != nil {
		return err
	}
	imageWithDigest, err := registry.GetImageTagWithDigest(options.Tag)
	if err != nil {
		return fmt.Errorf("error accessing image at registry %s: %v", options.Tag, err)
	}

	oktetoLog.Success("Image for service '%s' pushed to registry: %s", service, options.Tag)
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
		oktetoLog.Information("Endpoints available:")
		oktetoLog.Printf("  - %s\n", strings.Join(eps, "\n  - "))
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

	if os.Getenv(model.GithubRepositoryEnvVar) == "" {
		repo, err := model.GetRepositoryURL(cwd)
		if err != nil {
			oktetoLog.Infof("could not retrieve repo name: %s", err)
		}
		os.Setenv(model.GithubRepositoryEnvVar, repo)
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

func checkServicesToBuild(service string, mOptions *model.BuildInfo, ch chan string) error {
	opts := build.OptsFromManifest(service, mOptions, build.BuildOptions{})

	if build.ShouldOptimizeBuild(opts.Tag) {
		oktetoLog.Debug("found OKTETO_GIT_COMMIT, optimizing the build flow")
		if skipBuild, err := checkImageAtGlobalAndSetEnvs(service, opts); err != nil {
			return err
		} else if skipBuild {
			oktetoLog.Information("Image for service '%s' already built at global registry: %s ", service, opts.Tag)
			return nil
		}
	}

	imageWithDigest, err := registry.GetImageTagWithDigest(opts.Tag)
	if err == oktetoErrors.ErrNotFound {
		oktetoLog.Debug("image not found, building image")
		ch <- service
		return nil
	} else if err != nil {
		return fmt.Errorf("error checking image at registry %s: %v", opts.Tag, err)
	}
	oktetoLog.Debug("Skipping build for image for service")

	return setManifestEnvVars(service, imageWithDigest)
}

func checkBuildFromManifest(ctx context.Context, buildManifest model.ManifestBuild) error {
	// check if images are at registry (global or dev) and set envs or send to build
	toBuild := make(chan string, len(buildManifest))
	g, _ := errgroup.WithContext(ctx)

	for service, mBuildInfo := range buildManifest {
		service, mBuildInfo := service, mBuildInfo
		g.Go(func() error {
			return checkServicesToBuild(service, mBuildInfo, toBuild)
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	close(toBuild)

	if len(toBuild) == 0 {
		oktetoLog.Information("Images were already built. To rebuild your images run 'okteto build' or 'okteto deploy --build'")
		return nil
	}

	for svc := range toBuild {
		oktetoLog.Warning("Image for service '%s' doesn't exist and it needs to be built", svc)
		oktetoLog.Information("To rebuild this image run 'okteto build %s' or 'okteto deploy --build'", svc)
		mBuildInfo := buildManifest[svc]
		if err := runBuildAndSetEnvs(ctx, svc, mBuildInfo); err != nil {
			return err
		}
	}

	return nil
}
