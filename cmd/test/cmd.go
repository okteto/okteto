package test

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	contextCMD "github.com/okteto/okteto/cmd/context"
	deployCMD "github.com/okteto/okteto/cmd/deploy"
	"github.com/okteto/okteto/cmd/namespace"
	pipelineCMD "github.com/okteto/okteto/cmd/pipeline"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	oktetoPath "github.com/okteto/okteto/pkg/path"
)

type Options struct {
	ManifestPath     string
	ManifestPathFlag string
	Namespace        string
	K8sContext       string
	Variables        []string
	Build            bool
	Timeout          time.Duration
	Wait             bool

	DeployName        string
	Dependencies      bool
	DeployWithoutBash bool
	DeployInRemote    bool
	DeployServices    []string
}

func Test(ctx context.Context, ioCtrl *io.Controller, k8sLogger *io.K8sLogger, at deployCMD.AnalyticsTrackerInterface) *cobra.Command {
	options := &Options{}
	cmd := &cobra.Command{
		Use:          "test",
		Short:        "Run tests",
		Hidden:       true,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, servicesToTest []string) error {
			stop := make(chan os.Signal, 1)
			signal.Notify(stop, os.Interrupt)
			exit := make(chan error, 1)

			go func() {
				exit <- doRun(ctx, servicesToTest, options, ioCtrl, k8sLogger, at)
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
	cmd.Flags().BoolVarP(&options.Build, "build", "", false, "force build of images before running the tests")
	cmd.Flags().StringArrayVarP(&options.Variables, "var", "v", []string{}, "set a variable (can be set more than once)")
	cmd.Flags().DurationVarP(&options.Timeout, "timeout", "t", getDefaultTimeout(), "the length of time to wait for completion, zero means never. Any other values should contain a corresponding time unit e.g. 1s, 2m, 3h ")
	cmd.Flags().BoolVarP(&options.Wait, "wait", "w", false, "wait until the development environment is deployed (defaults to false)")
	cmd.Flags().BoolVarP(&options.Dependencies, "dependencies", "", false, "deploy the dependencies from manifest")

	cmd.Flags().StringVar(&options.DeployName, "deploy-name", "", "name of the development environment name to be deployed")
	cmd.Flags().StringArrayVar(&options.DeployServices, "deploy-services", []string{}, "name of the services of the development environment that should be deployed")
	cmd.Flags().BoolVarP(&options.DeployWithoutBash, "deploy-no-bash", "", false, "execute commands without bash")
	cmd.Flags().BoolVarP(&options.DeployInRemote, "deploy-remote", "", false, "force run deploy commands in remote")

	return cmd
}

func doRun(ctx context.Context, servicesToTest []string, options *Options, ioCtrl *io.Controller, k8sLogger *io.K8sLogger, at deployCMD.AnalyticsTrackerInterface) error {
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

	if options.ManifestPath != "" {
		// if path is absolute, its transformed to rel from root
		initialCWD, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get the current working directory: %w", err)
		}
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

	c := deployCMD.Command{
		GetManifest: func(path string, fs afero.Fs) (*model.Manifest, error) {
			return manifest, nil
		},
		K8sClientProvider:  k8sClientProvider,
		Builder:            buildv2.NewBuilderFromScratch(at, ioCtrl),
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
		Name:             options.DeployName,
		Namespace:        options.Namespace,
		K8sContext:       options.K8sContext,
		ServicesToDeploy: options.DeployServices,
		Variables:        options.Variables,
		Build:            options.Build,
		Dependencies:     options.Dependencies,
		Timeout:          options.Timeout,
		RunWithoutBash:   options.DeployWithoutBash,
		RunInRemote:      options.DeployInRemote,
		Wait:             options.Wait,
		ShowCTA:          false,
	}); err != nil {
		oktetoLog.Error("deploy failed: ", err.Error())
		return err
	}

	manifest.ExpandEnvVars()

	tt := Tester{
		Manifest: manifest,
	}
	return tt.Run(ctx)
}
