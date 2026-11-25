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
	"strings"
	"time"

	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	contextCMD "github.com/okteto/okteto/cmd/context"
	deployCMD "github.com/okteto/okteto/cmd/deploy"
	"github.com/okteto/okteto/cmd/namespace"
	pipelineCMD "github.com/okteto/okteto/cmd/pipeline"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/build/buildkit"
	buildCMD "github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/dag"
	"github.com/okteto/okteto/pkg/deployable"
	"github.com/okteto/okteto/pkg/devenvironment"
	"github.com/okteto/okteto/pkg/divert"
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
	"github.com/okteto/okteto/pkg/repository"
	"github.com/okteto/okteto/pkg/types"
	"github.com/okteto/okteto/pkg/validator"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

type Options struct {
	ManifestPath     string
	ManifestPathFlag string
	Namespace        string
	K8sContext       string
	Name             string
	Variables        []string
	// Commands are the values of the --command flag, which replace the commands of the selected test container
	Commands []string
	// Profiles are the values of the --profile flag, which enable the test containers that have any of them
	Profiles []string
	Timeout  time.Duration
	Deploy   bool
	NoCache  bool
	// override is the command override resolved from the '--' arguments or the --command flag
	override commandOverride
	// GitCredentialsEndpoint is the URL the Git credential helper gets the credentials from that Okteto sets on its
	// installer. It is read from the env before any variable is loaded
	GitCredentialsEndpoint string
	// enabledProfiles are the profiles resolved from the --profile flag or the OKTETO_PROFILES environment variable
	enabledProfiles []string
}

type builder interface {
	GetSvcToBuildFromRegex(manifest *model.Manifest, imgFinder model.ImageFromManifest) (string, error)
	GetServicesToBuildDuringExecution(ctx context.Context, manifest *model.Manifest, svcsToDeploy []string) ([]string, error)
	Build(ctx context.Context, options *types.BuildOptions) error
}

type insightsTracker interface {
	TrackTest(ctx context.Context, meta *analytics.SingleTestMetadata)
}

func Test(ctx context.Context, ioCtrl *io.Controller, k8sLogger *io.K8sLogger, at *analytics.Tracker, insights insightsTracker) *cobra.Command {
	options := &Options{}
	cmd := &cobra.Command{
		Use:                   "test [testName...] [flags] [-- COMMAND [args...]]",
		DisableFlagsInUseLine: true,
		Short:                 "Run tests using Remote Execution",
		Example: `# Run the test containers without profiles defined in the Okteto Manifest
okteto test

# Run the test containers without profiles and the ones with the 'pre-commit' profile
okteto test --profile pre-commit

# Run the 'unit' test container replacing its commands with a single one
okteto test unit -- go test ./pkg/api -run TestLogin

# Run the 'unit' test container replacing its commands with several ones, run in order
okteto test unit --command "go mod download" --command "go test ./pkg/api -run TestLogin"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.GitCredentialsEndpoint = remote.GitCredentialsEndpointFromEnv()

			if err := validator.CheckReservedVariablesNameOption(options.Variables); err != nil {
				return err
			}

			// The profiles are checked before connecting to the cluster with the environment of the process.
			// They are resolved again once the context loads the .env file and the Okteto Variables
			profiles, err := resolveProfiles(options.Profiles, os.Getenv(constants.OktetoProfilesEnvVar))
			if err != nil {
				return err
			}

			// The profiles are checked before the override, so using both reports the conflict
			// instead of the missing test container name required by the override
			overrideRequested := cmd.ArgsLenAtDash() >= 0 || len(options.Commands) > 0
			if err := profiles.checkCommandOverride(overrideRequested); err != nil {
				return err
			}

			testContainers, override, err := parseTestArgs(args, cmd.ArgsLenAtDash(), options.Commands)
			if err != nil {
				return err
			}
			if err := profiles.checkTestNames(testContainers); err != nil {
				return err
			}
			options.override = override

			stop := make(chan os.Signal, 1)
			signal.Notify(stop, os.Interrupt)
			exit := make(chan error, 1)

			go func() {
				startTime := time.Now()
				metadata, err := doRun(ctx, testContainers, options, ioCtrl, k8sLogger, &ProxyTracker{at}, insights, at)
				metadata.Err = err
				metadata.Duration = time.Since(startTime)
				metadata.CommandOverride = override.isSet()
				metadata.ProfileSelected = len(options.enabledProfiles) > 0
				at.TrackTest(metadata)
				exit <- err
			}()
			select {
			case <-stop:
				oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
				oktetoLog.Spinner("Shutting down...")
				oktetoLog.StartSpinner()
				defer oktetoLog.StopSpinner()
				deployable.CleanUpPlatformFiles()
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
	cmd.Flags().StringArrayVar(&options.Commands, "command", []string{}, "replace the commands of the test container, which must be the only one named (can be set more than once, run in order)")
	cmd.Flags().StringArrayVar(&options.Profiles, "profile", []string{}, "enable a profile to run its test containers along with the ones without profiles (can be set more than once). The OKTETO_PROFILES environment variable overrides it")
	cmd.Flags().DurationVarP(&options.Timeout, "timeout", "t", getDefaultTimeout(), "the duration to wait for the Test Container to run. Any value should contain a corresponding time unit e.g. 1s, 2m, 3h")
	cmd.Flags().StringVar(&options.Name, "name", "", "the name of the Development Environment")
	cmd.Flags().BoolVar(&options.Deploy, "deploy", false, "Force execution of the commands in the 'deploy' section")
	cmd.Flags().BoolVar(&options.NoCache, "no-cache", false, "by default, the caches of a Test Container are reused between executions")

	return cmd
}

func doRun(ctx context.Context, servicesToTest []string, options *Options, ioCtrl *io.Controller, k8sLogger *io.K8sLogger, tracker *ProxyTracker, at insightsTracker, connTracker *analytics.Tracker) (analytics.TestMetadata, error) {
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

	// The context loads the .env file and the Okteto Variables into the environment, so they can set OKTETO_PROFILES
	if err := resolveEnabledProfiles(servicesToTest, options, ioCtrl); err != nil {
		return analytics.TestMetadata{}, err
	}

	create, err := utils.ShouldCreateNamespace(ctx, okteto.GetContext().Namespace)
	if err != nil {
		return analytics.TestMetadata{}, err
	}
	if create {
		nsCmd, err := namespace.NewCommand(ioCtrl)
		if err != nil {
			return analytics.TestMetadata{}, err
		}
		if err := nsCmd.Create(ctx, &namespace.CreateOptions{Namespace: okteto.GetContext().Namespace}); err != nil {
			return analytics.TestMetadata{}, err
		}
	}

	if options.ManifestPath != "" {
		// if path is absolute, it's transformed to rel from root
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
			oktetoLog.Information("There are no tests configured in your Okteto Manifest. For more information, check the documentation: https://www.okteto.com/docs/reference/okteto-manifest/#test")
			return analytics.TestMetadata{}, nil
		}

		return analytics.TestMetadata{}, err
	}

	// The test containers to run are shown when they are selected by profile, as some of them may not run
	showSelection := len(servicesToTest) == 0 && len(definedProfiles(manifest.Test)) > 0
	servicesToTest, err = selectTests(manifest.Test, servicesToTest, options.enabledProfiles)
	if err != nil {
		return analytics.TestMetadata{}, err
	}

	k8sClientProvider := okteto.NewK8sClientProviderWithLogger(k8sLogger)

	pc, err := pipelineCMD.NewCommand(connTracker)
	if err != nil {
		return analytics.TestMetadata{}, fmt.Errorf("could not create pipeline command: %w", err)
	}

	configmapHandler := deployCMD.NewConfigmapHandler(k8sClientProvider, k8sLogger)

	okCtxForBuilder := &okteto.ContextStateless{
		Store: okteto.GetContextStore(),
	}
	conn := buildCMD.GetBuildkitConnector(okCtxForBuilder, ioCtrl, connTracker)
	builder := buildv2.NewBuilderFromScratch(ioCtrl, []buildv2.OnBuildFinish{
		tracker.TrackImageBuild,
	}, conn)

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
	if showSelection {
		ioCtrl.Out().Infof("Test containers to run: %s", strings.Join(testServices, ", "))
	}

	wasBuilt, err := doBuild(ctx, manifest, testServices, builder, ioCtrl)
	if err != nil {
		return analytics.TestMetadata{}, fmt.Errorf("okteto test needs to build the images defined but failed: %w", err)
	}

	if options.Deploy && manifest.Deploy == nil {
		oktetoLog.Warning("Nothing to deploy. The 'deploy' section of your Okteto Manifest is empty. For more information, check the docs: https://www.okteto.com/docs/reference/okteto-manifest/#deploy")
	} else if options.Deploy && manifest.Deploy != nil {
		c := deployCMD.Command{
			GetManifest: func(path string, fs afero.Fs) (*model.Manifest, error) {
				return manifest, nil
			},
			K8sClientProvider:  k8sClientProvider,
			Builder:            builder,
			RemoteConnector:    conn,
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
			DivertDeployerGetter: func(d *model.DivertDeploy, name, namespace string, c kubernetes.Interface, ioCtrl *io.Controller) (deployCMD.DivertDeployer, error) {
				return divert.New(d, name, namespace, c, ioCtrl)
			},
		}
		// runInRemote is not set at init, it's left default to false and calculated by deployCMD.ShouldRunInRemote
		opts := &deployCMD.Options{
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
			Wait:             true,
			ShowCTA:          false,
		}
		deployStartTime := time.Now()
		// we need to pre-calculate the value of the runInRemote flag before running the deploy command
		// in order to track the information correctly, using the same logic as the deploy command
		runInRemote := deployCMD.ShouldRunInRemote(opts)
		err = c.Run(ctx, opts)
		c.TrackDeploy(manifest, runInRemote, deployStartTime, err, opts.Namespace)

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

	testAnalytics := make([]*analytics.SingleTestMetadata, 0)

	// send all events appended on each test

	defer func([]*analytics.SingleTestMetadata) {
		for _, meta := range testAnalytics {
			m := meta
			at.TrackTest(ctx, m)
		}
	}(testAnalytics)

	for _, name := range testServices {
		overridden := options.override.appliesTo(name)
		test := options.override.resolveTest(name, manifest.Test[name])

		ctxCwd := path.Clean(path.Join(cwd, test.Context))

		commandFlags, err := deployCMD.GetCommandFlags(name, options.Variables)
		if err != nil {
			return metadata, err
		}

		runner := remote.NewRunner(ioCtrl, buildCMD.NewOktetoBuilder(
			okCtxForBuilder,
			fs,
			ioCtrl,
			conn,
		))
		commands := make([]model.DeployCommand, len(test.Commands))

		for i, cmd := range test.Commands {
			commands[i] = model.DeployCommand(cmd)
		}

		ig := ignore.NewOktetoIgnorer(path.Join(ctxCwd, model.IgnoreFilename))

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
			TestName:                    name,
			Caches:                      test.Caches,
			IgnoreRules:                 testIgnoreRules,
			Artifacts:                   test.Artifacts,
			Hosts:                       test.Hosts,
			GitCredentialsEndpoint:      options.GitCredentialsEndpoint,
		}
		for k, v := range deployable.GetGatewayEnvironment() {
			params.OktetoCommandSpecificEnvVars[k] = v
		}
		if test.SkipIfNoFileChanges && !options.NoCache {
			params.CacheInvalidationKey = "const"
		}

		if overridden {
			ioCtrl.Out().Infof("Executing test container '%s' with overridden commands", name)
			for _, command := range test.Commands {
				ioCtrl.Logger().Debugf("Overridden command for test container '%s': %s", name, command.Command)
			}
		} else {
			ioCtrl.Out().Infof("Executing test container '%s'", name)
		}
		testMetadata := analytics.SingleTestMetadata{
			DevenvName: manifest.Name,
			TestName:   name,
			Namespace:  okteto.GetContext().Namespace,
			Repository: repository.NewRepository(manifest.ManifestPath).GetAnonymizedRepo(),
		}
		testStartTime := time.Now()
		err = runner.Run(ctx, params)
		testMetadata.Duration = time.Since(testStartTime)
		testMetadata.Success = err == nil
		// A run with overridden commands is not a run of the test container defined in the
		// manifest, so it is not published to Insights to keep the test metrics accurate
		if !overridden {
			testAnalytics = append(testAnalytics, &testMetadata)
		}
		if err != nil {
			// If it is a commandErr, it means that there was an error in the tests themselves, so we should return that error directly
			var cmdErr buildkit.CommandErr
			if errors.As(err, &cmdErr) {
				return metadata, err
			}

			hint := `Please verify the specified image is accessible.
    You can use --log-level flag to get additional output.`
			if len(test.Artifacts) > 0 {
				hint = `Please verify the specified image is accessible and review if expected artifacts are being generated.
    You can use --log-level flag to get additional output.`
			}

			ioCtrl.Logger().Infof("error executing test container: %v", err)
			err = oktetoErrors.UserError{
				E:    fmt.Errorf("error executing test container '%s'", name),
				Hint: hint,
			}
			return metadata, err
		}
		oktetoLog.Success("Test container '%s' passed", name)
	}
	metadata.Success = true
	return metadata, nil
}

// resolveEnabledProfiles sets the enabled profiles from the --profile flag or the OKTETO_PROFILES environment
// variable, checking that they are not combined with test container names or a command override
func resolveEnabledProfiles(names []string, options *Options, ioCtrl *io.Controller) error {
	profiles, err := resolveProfiles(options.Profiles, os.Getenv(constants.OktetoProfilesEnvVar))
	if err != nil {
		return err
	}
	if err := profiles.checkCommandOverride(options.override.isSet()); err != nil {
		return err
	}
	if err := profiles.checkTestNames(names); err != nil {
		return err
	}

	if profiles.fromEnv() {
		ioCtrl.Out().Infof("Using the profiles from %s: %s", constants.OktetoProfilesEnvVar, strings.Join(profiles.profiles, ", "))
		if len(options.Profiles) > 0 {
			ioCtrl.Out().Warning("%s overrides the values of '--profile'", constants.OktetoProfilesEnvVar)
		}
	}
	options.enabledProfiles = profiles.profiles
	return nil
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
