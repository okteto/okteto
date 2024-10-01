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
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path"
	"time"

	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	contextCMD "github.com/okteto/okteto/cmd/context"
	deployCMD "github.com/okteto/okteto/cmd/deploy"
	"github.com/okteto/okteto/cmd/namespace"
	pipelineCMD "github.com/okteto/okteto/cmd/pipeline"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	buildCMD "github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/dag"
	"github.com/okteto/okteto/pkg/deployable"
	"github.com/okteto/okteto/pkg/devenvironment"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/okteto/okteto/pkg/ignore"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	oktetoPath "github.com/okteto/okteto/pkg/path"
	"github.com/okteto/okteto/pkg/remote"
	"github.com/okteto/okteto/pkg/types"
	"github.com/okteto/okteto/pkg/validator"
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
	Deploy           bool
	NoCache          bool
}

type builder interface {
	GetSvcToBuildFromRegex(manifest *model.Manifest, imgFinder model.ImageFromManifest) (string, error)
	GetServicesToBuildDuringExecution(ctx context.Context, manifest *model.Manifest, svcsToDeploy []string) ([]string, error)
	Build(ctx context.Context, options *types.BuildOptions) error
}

func Test(ctx context.Context, ioCtrl *io.Controller, k8sLogger *io.K8sLogger, at *analytics.Tracker) *cobra.Command {
	options := &Options{}
	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run tests using Remote Execution",
		RunE: func(cmd *cobra.Command, servicesToTest []string) error {

			if err := validator.CheckReservedVariablesNameOption(options.Variables); err != nil {
				return err
			}

			stop := make(chan os.Signal, 1)
			signal.Notify(stop, os.Interrupt)
			exit := make(chan error, 1)

			go func() {
				startTime := time.Now()
				metadata, err := doRun(ctx, servicesToTest, options, ioCtrl, k8sLogger, &ProxyTracker{at})
				metadata.Err = err
				metadata.Duration = time.Since(startTime)
				at.TrackTest(metadata)
				exit <- err
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

	cmd.Flags().StringVarP(&options.ManifestPath, "file", "f", "", "the path to the Okteto Manifest")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "overwrite the current Okteto Namespace")
	cmd.Flags().StringVarP(&options.K8sContext, "context", "c", "", "overwrite the current Okteto Context")
	cmd.Flags().StringArrayVarP(&options.Variables, "var", "v", []string{}, "set a variable to be injected in the test commands (can be set more than once)")
	cmd.Flags().DurationVarP(&options.Timeout, "timeout", "t", getDefaultTimeout(), "the duration to wait for the Test Container to run. Any value should contain a corresponding time unit e.g. 1s, 2m, 3h")
	cmd.Flags().StringVar(&options.Name, "name", "", "the name of the Development Environment")
	cmd.Flags().BoolVar(&options.Deploy, "deploy", false, "Force execution of the commands in the 'deploy' section")
	cmd.Flags().BoolVar(&options.NoCache, "no-cache", false, "by default, the caches of a Test Container are reused between executions")

	return cmd
}

func doRun(ctx context.Context, servicesToTest []string, options *Options, ioCtrl *io.Controller, k8sLogger *io.K8sLogger, tracker *ProxyTracker) (analytics.TestMetadata, error) {
	fs := afero.NewOsFs()

	ctxOpts := &contextCMD.Options{
		Context:   options.K8sContext,
		Namespace: options.Namespace,
		Show:      true,
	}
	if err := contextCMD.NewContextCommand().Run(ctx, ctxOpts); err != nil {
		return analytics.TestMetadata{}, err
	}

	if !okteto.IsOkteto() {
		return analytics.TestMetadata{}, oktetoErrors.ErrContextIsNotOktetoCluster
	}

	create, err := utils.ShouldCreateNamespace(ctx, okteto.GetContext().Namespace)
	if err != nil {
		return analytics.TestMetadata{}, err
	}
	if create {
		nsCmd, err := namespace.NewCommand()
		if err != nil {
			return analytics.TestMetadata{}, err
		}
		if err := nsCmd.Create(ctx, &namespace.CreateOptions{Namespace: okteto.GetContext().Namespace}); err != nil {
			return analytics.TestMetadata{}, err
		}
	}

	if options.ManifestPath != "" {
		// if path is absolute, its transformed to rel from root
		initialCWD, err := os.Getwd()
		if err != nil {
			return analytics.TestMetadata{}, fmt.Errorf("failed to get the current working directory: %w", err)
		}
		manifestPathFlag, err := oktetoPath.GetRelativePathFromCWD(initialCWD, options.ManifestPath)
		if err != nil {
			return analytics.TestMetadata{}, err
		}
		// as the installer uses root for executing the pipeline, we save the rel path from root as ManifestPathFlag option
		options.ManifestPathFlag = manifestPathFlag

		if err := validator.FileArgumentIsNotDir(fs, options.ManifestPath); err != nil {
			return analytics.TestMetadata{}, err
		}

		// when the manifest path is set by the cmd flag, we are moving cwd so the cmd is executed from that dir
		uptManifestPath, err := filesystem.UpdateCWDtoManifestPath(options.ManifestPath)
		if err != nil {
			return analytics.TestMetadata{}, err
		}
		options.ManifestPath = uptManifestPath
	}

	manifest, err := model.GetManifestV2(options.ManifestPath, fs)
	if err != nil {
		return analytics.TestMetadata{}, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return analytics.TestMetadata{}, fmt.Errorf("failed to get the current working directory to resolve name: %w", err)
	}

	if manifest.Name == "" {
		c, _, err := okteto.NewK8sClientProvider().Provide(okteto.GetContext().Cfg)
		if err != nil {
			return analytics.TestMetadata{}, err
		}
		inferer := devenvironment.NewNameInferer(c)
		manifest.Name = inferer.InferName(ctx, cwd, okteto.GetContext().Namespace, options.ManifestPath)
	}

	if err := manifest.Test.Validate(); err != nil {
		if errors.Is(err, model.ErrNoTestsDefined) {
			oktetoLog.Information("There are no tests configured in your Okteto Manifest. For more information, check the documentation: https://okteto.com/docs/core/okteto-manifest/#test")
			return analytics.TestMetadata{}, nil
		}

		return analytics.TestMetadata{}, err
	}

	k8sClientProvider := okteto.NewK8sClientProviderWithLogger(k8sLogger)

	pc, err := pipelineCMD.NewCommand()
	if err != nil {
		return analytics.TestMetadata{}, fmt.Errorf("could not create pipeline command: %w", err)
	}

	configmapHandler := deployCMD.NewConfigmapHandler(k8sClientProvider, k8sLogger)

	builder := buildv2.NewBuilderFromScratch(ioCtrl, []buildv2.OnBuildFinish{
		tracker.TrackImageBuild,
	})

	var nodes []dag.Node

	for name, test := range manifest.Test {
		nodes = append(nodes, Node{test, name})
	}

	tree, err := dag.From(nodes...)
	if err != nil {
		return analytics.TestMetadata{}, err
	}

	tree, err = tree.Subtree(servicesToTest...)
	if err != nil {
		return analytics.TestMetadata{}, err
	}

	testServices := tree.Ordered()

	wasBuilt, err := doBuild(ctx, manifest, testServices, builder, ioCtrl)
	if err != nil {
		return analytics.TestMetadata{}, fmt.Errorf("okteto test needs to build the images defined but failed: %w", err)
	}

	shouldDeploy := options.Deploy

	if shouldDeploy && manifest.Deploy == nil {
		oktetoLog.Warning("Nothing to deploy. The 'deploy' section of your Okteto Manifest is empty. For more information, check the docs: https://okteto.com/docs/core/okteto-manifest/#deploy")
		shouldDeploy = false
	}

	if shouldDeploy {
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
			AnalyticsTracker:   tracker,
			IoCtrl:             ioCtrl,
			K8sLogger:          k8sLogger,
			IsRemote:           env.LoadBoolean(constants.OktetoDeployRemote),
			RunningInInstaller: config.RunningInInstaller(),
		}
		deployStartTime := time.Now()
		runInRemote := shouldRunInRemote(manifest.Deploy)
		err = c.Run(ctx, &deployCMD.Options{
			Manifest:         manifest,
			ManifestPathFlag: options.ManifestPathFlag,
			ManifestPath:     options.ManifestPath,
			Name:             options.Name,
			Namespace:        okteto.GetContext().Namespace,
			K8sContext:       okteto.GetContext().Name,
			Variables:        options.Variables,
			NoBuild:          false,
			Dependencies:     false,
			Timeout:          options.Timeout,
			RunWithoutBash:   false,
			RunInRemote:      runInRemote,
			Wait:             true,
			ShowCTA:          false,
		})
		c.TrackDeploy(manifest, runInRemote, deployStartTime, err)

		if err != nil {
			oktetoLog.Errorf("deploy failed: %s", err.Error())
			return analytics.TestMetadata{}, err
		}
	} else {
		// The deploy operation expands environment variables in the manifest. If
		// we don't deploy, make sure to expand the envvars
		if err := manifest.ExpandEnvVars(); err != nil {
			return analytics.TestMetadata{}, fmt.Errorf("failed to expand manifest environment variables: %w", err)
		}
	}

	metadata := analytics.TestMetadata{
		StagesCount: len(testServices),
		Deployed:    options.Deploy,
		WasBuilt:    wasBuilt,
	}

	for _, name := range testServices {
		test := manifest.Test[name]

		ctxCwd := path.Clean(path.Join(cwd, test.Context))

		commandFlags, err := deployCMD.GetCommandFlags(name, options.Variables)
		if err != nil {
			return metadata, err
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

		ig, err := ignore.NewFromFile(path.Join(ctxCwd, model.IgnoreFilename))
		if err != nil {
			return analytics.TestMetadata{}, fmt.Errorf("failed to read ignore file: %w", err)
		}

		// Read "test" and "test.{name}" sections from the .oktetoignore file
		testIgnoreRules, err := ig.Rules(ignore.RootSection, "test", fmt.Sprintf("test.%s", name))
		if err != nil {
			return analytics.TestMetadata{}, fmt.Errorf("failed to create ignore rules for %s: %w", name, err)
		}
		params := &remote.Params{
			BaseImage:           test.Image,
			ManifestPathFlag:    options.ManifestPathFlag,
			TemplateName:        "dockerfile",
			CommandFlags:        commandFlags,
			BuildEnvVars:        builder.GetBuildEnvVars(),
			DependenciesEnvVars: deployCMD.GetDependencyEnvVars(os.Environ),
			ExecutionEnvVars:    deployable.GetPlatformEnvironment(ctx),
			DockerfileName:      "Dockerfile.test",
			OktetoCommandSpecificEnvVars: map[string]string{
				constants.OktetoIsPreviewEnvVar: os.Getenv(constants.OktetoIsPreviewEnvVar),
				constants.CIEnvVar: func() string {
					if val, ok := os.LookupEnv(constants.CIEnvVar); ok {
						return val
					}
					return "true"
				}(),
			},
			Deployable: deployable.Entity{
				Commands: commands,
				// Added this for backward compatibility. Before the refactor we were having the env variables for the external
				// resources in the environment, so including it to set the env vars in the remote-run
				External: manifest.External,
			},
			Manifest:                    manifest,
			Command:                     remote.TestCommand,
			ContextAbsolutePathOverride: ctxCwd,
			Caches:                      test.Caches,
			IgnoreRules:                 testIgnoreRules,
			Artifacts:                   test.Artifacts,
			UseRootUser:                 true,
			NoCache:                     options.NoCache,
			Hosts:                       test.Hosts,
		}

		ioCtrl.Out().Infof("Executing test container '%s'", name)
		if err := runner.Run(ctx, params); err != nil {
			return metadata, err
		}
		oktetoLog.Success("Test container '%s' passed", name)
	}
	metadata.Success = true
	return metadata, nil
}

func shouldRunInRemote(manifestDeploy *model.DeployInfo) bool {
	if manifestDeploy != nil {
		if manifestDeploy.Image != "" || manifestDeploy.Remote {
			return true
		}
	}

	return false
}

func doBuild(ctx context.Context, manifest *model.Manifest, svcs []string, builder builder, ioCtrl *io.Controller) (bool, error) {
	// make sure the images used for the tests exist. If they don't build them
	svcsToBuild := []string{}
	for _, name := range svcs {
		imgName := manifest.Test[name].Image
		getImg := func(manifest *model.Manifest) string { return imgName }
		svc, err := builder.GetSvcToBuildFromRegex(manifest, getImg)
		if err != nil {
			switch {
			case errors.Is(err, buildv2.ErrOktetBuildSyntaxImageIsNotInBuildSection):
				return false, fmt.Errorf("test '%s' needs image '%s' but it's not defined in the build section of the Okteto Manifest. See: https://www.okteto.com/docs/core/okteto-variables/#built-in-environment-variables-for-images-in-okteto-registry", name, imgName)
			case errors.Is(err, buildv2.ErrImageIsNotAOktetoBuildSyntax):
				ioCtrl.Logger().Debugf("error getting services to build for image '%s': %s", imgName, err)
				continue
			default:
				return false, fmt.Errorf("failed to get services to build: %w", err)
			}

		}
		svcsToBuild = append(svcsToBuild, svc)
	}

	if len(svcsToBuild) > 0 {
		servicesNotBuild, err := builder.GetServicesToBuildDuringExecution(ctx, manifest, svcsToBuild)
		if err != nil {
			return false, fmt.Errorf("failed to get services to build: %w", err)
		}

		if len(servicesNotBuild) != 0 {
			buildOptions := &types.BuildOptions{
				EnableStages: true,
				Manifest:     manifest,
				CommandArgs:  servicesNotBuild,
			}

			if errBuild := builder.Build(ctx, buildOptions); errBuild != nil {
				return true, errBuild
			}
			return true, nil
		}
	}
	return false, nil
}
