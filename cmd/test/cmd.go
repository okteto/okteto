// Copyright 2024 The Okteto Authors
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
package test

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	contextCMD "github.com/okteto/okteto/cmd/context"
	deployCMD "github.com/okteto/okteto/cmd/deploy"
	"github.com/okteto/okteto/cmd/namespace"
	pipelineCMD "github.com/okteto/okteto/cmd/pipeline"
	"github.com/okteto/okteto/cmd/utils"
	buildCMD "github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/dag"
	"github.com/okteto/okteto/pkg/deployable"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	oktetoPath "github.com/okteto/okteto/pkg/path"
	"github.com/okteto/okteto/pkg/remote"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type Options struct {
	ManifestPath     string
	ManifestPathFlag string
	Namespace        string
	K8sContext       string
	Name             string
	Variables        []string
	Timeout          time.Duration
}

func Test(ctx context.Context, ioCtrl *io.Controller, k8sLogger *io.K8sLogger, at deployCMD.AnalyticsTrackerInterface) *cobra.Command {
	options := &Options{}
	cmd := &cobra.Command{
		Use:          "test",
		Short:        "Run tests",
		Hidden:       true,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			stop := make(chan os.Signal, 1)
			signal.Notify(stop, os.Interrupt)
			exit := make(chan error, 1)

			go func() {
				exit <- doRun(ctx, options, ioCtrl, k8sLogger, at)
			}()
			select {
			case <-stop:
				oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
				oktetoLog.Spinner("Shutting down...")
				oktetoLog.StartSpinner()
				defer oktetoLog.StopSpinner()
				return oktetoErrors.ErrIntSig
			case err := <-exit:
				return err
			}

		},
	}

	cmd.Flags().StringVarP(&options.ManifestPath, "file", "f", "", "path to the okteto manifest file")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "overwrites the namespace where the development environment is deployed")
	cmd.Flags().StringVarP(&options.K8sContext, "context", "c", "", "context where the development environment is deployed")
	cmd.Flags().StringArrayVarP(&options.Variables, "var", "v", []string{}, "set a variable (can be set more than once)")
	cmd.Flags().DurationVarP(&options.Timeout, "timeout", "t", getDefaultTimeout(), "the length of time to wait for completion, zero means never. Any other values should contain a corresponding time unit e.g. 1s, 2m, 3h ")
	cmd.Flags().StringVar(&options.Name, "name", "", "name of the development environment name to be deployed")

	return cmd
}

func doRun(ctx context.Context, options *Options, ioCtrl *io.Controller, k8sLogger *io.K8sLogger, at deployCMD.AnalyticsTrackerInterface) error {
	fs := afero.NewOsFs()

	// Loads, updates and uses the context from path. If not found, it creates and uses a new context
	if err := contextCMD.LoadContextFromPath(ctx, options.Namespace, options.K8sContext, options.ManifestPath, contextCMD.Options{Show: true}); err != nil {
		if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.Options{Namespace: options.Namespace}); err != nil {
			return err
		}
	}

	if !okteto.IsOkteto() {
		return fmt.Errorf("'okteto test' is only supported in contexts that have Okteto installed")
	}

	create, err := utils.ShouldCreateNamespace(ctx, okteto.GetContext().Namespace)
	if err != nil {
		return err
	}
	if create {
		nsCmd, err := namespace.NewCommand()
		if err != nil {
			return err
		}
		if err := nsCmd.Create(ctx, &namespace.CreateOptions{Namespace: okteto.GetContext().Namespace}); err != nil {
			return err
		}
	}
	// if path is absolute, its transformed to rel from root
	initialCWD, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get the current working directory: %w", err)
	}

	if options.ManifestPath != "" {
		manifestPathFlag, err := oktetoPath.GetRelativePathFromCWD(initialCWD, options.ManifestPath)
		if err != nil {
			return err
		}
		// as the installer uses root for executing the pipeline, we save the rel path from root as ManifestPathFlag option
		options.ManifestPathFlag = manifestPathFlag

		// when the manifest path is set by the cmd flag, we are moving cwd so the cmd is executed from that dir
		uptManifestPath, err := model.UpdateCWDtoManifestPath(options.ManifestPath)
		if err != nil {
			return err
		}
		options.ManifestPath = uptManifestPath
	}

	manifest, err := model.GetManifestV2(options.ManifestPath, fs)
	if err != nil {
		return err
	}

	k8sClientProvider := okteto.NewK8sClientProviderWithLogger(k8sLogger)

	pc, err := pipelineCMD.NewCommand()
	if err != nil {
		return fmt.Errorf("could not create pipeline command: %w", err)
	}

	configmapHandler := deployCMD.NewConfigmapHandler(k8sClientProvider, k8sLogger)

	builder := buildv2.NewBuilderFromScratch(ioCtrl, nil)

	kubeClient, _, err := k8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		return fmt.Errorf("could not instantiate kuberentes client: %w", err)
	}

	namer := deployCMD.Namer{
		KubeClient:   kubeClient,
		Workdir:      initialCWD,
		ManifestPath: options.ManifestPathFlag,
		ManifestName: options.Name,
	}

	name := options.Name
	if name == "" {
		name = namer.ResolveName(ctx)
	}

	namespace := manifest.Namespace
	if namespace == "" {
		namespace = okteto.GetContext().Namespace
	}

	if !pipeline.IsDeployed(ctx, name, namespace, kubeClient) {
		c := deployCMD.Command{
			GetManifest: func(path string, fs afero.Fs) (*model.Manifest, error) {
				return manifest, nil
			},
			K8sClientProvider:  k8sClientProvider,
			Builder:            builder,
			GetDeployer:        deployCMD.GetDeployer,
			EndpointGetter:     deployCMD.NewEndpointGetter,
			DeployWaiter:       deployCMD.NewDeployWaiter(k8sClientProvider, k8sLogger),
			CfgMapHandler:      configmapHandler,
			Fs:                 fs,
			PipelineCMD:        pc,
			AnalyticsTracker:   at,
			IoCtrl:             ioCtrl,
			K8sLogger:          k8sLogger,
			IsRemote:           env.LoadBoolean(constants.OktetoDeployRemote),
			RunningInInstaller: config.RunningInInstaller(),
		}
		if err = c.Run(ctx, &deployCMD.Options{
			Manifest:         manifest,
			ManifestPathFlag: options.ManifestPathFlag,
			ManifestPath:     options.ManifestPath,
			Name:             options.Name,
			Namespace:        options.Namespace,
			K8sContext:       options.K8sContext,
			Variables:        options.Variables,
			Build:            false,
			Dependencies:     false,
			Timeout:          options.Timeout,
			RunWithoutBash:   false,
			RunInRemote:      manifest.Deploy.Remote,
			Wait:             true,
			ShowCTA:          false,
		}); err != nil {
			oktetoLog.Error("deploy failed: %s", err.Error())
			return err
		}
	} else {
		// The deploy operation expand environment variables in the manifest. If
		// we don't deploy, make sure to expand the envvars
		if err := manifest.ExpandEnvVars(); err != nil {
			return fmt.Errorf("failed to expand manifest environment variables.: %w", err)
		}
		oktetoLog.Information("'%s' was already deployed. To redeploy run 'okteto deploy'", name)
	}

	var nodes []dag.Node

	for name, test := range manifest.Test {
		nodes = append(nodes, Node{test, name})
	}

	tree, err := dag.From(nodes...)
	if err != nil {
		return err
	}

	for _, name := range tree.Ordered() {
		test := manifest.Test[name]

		commandsFlags, err := deployCMD.GetCommandFlags(name, options.Variables)
		if err != nil {
			return err
		}

		runner := remote.NewRunner(ioCtrl, buildCMD.NewOktetoBuilder(
			&okteto.ContextStateless{
				Store: okteto.GetContextStore(),
			},
			fs,
		))
		commands := make([]model.DeployCommand, len(test.Commands))

		for i, cmd := range test.Commands {
			commands[i] = model.DeployCommand(cmd)
		}

		params := &remote.Params{
			BaseImage:           test.Image,
			ManifestPathFlag:    options.ManifestPathFlag,
			TemplateName:        "dockerfile",
			CommandFlags:        commandsFlags,
			BuildEnvVars:        builder.GetBuildEnvVars(),
			DependenciesEnvVars: deployCMD.GetDependencyEnvVars(os.Environ),
			DockerfileName:      "Dockerfile.test",
			Deployable: deployable.Entity{
				Commands: commands,
				// Added this for backward compatibility. Before the refactor we were having the env variables for the external
				// resources in the environment, so including it to set the env vars in the remote-run
				External: manifest.External,
			},
			Manifest: manifest,
			Command:  remote.TestCommand,
		}

		ioCtrl.Logger().Infof("Executing test for: %s", name)
		if err := runner.Run(ctx, params); err != nil {
			return err
		}
	}

	return nil
}
