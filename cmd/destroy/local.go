package destroy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"

	pipelineCMD "github.com/okteto/okteto/cmd/pipeline"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

type localDestroyCommand struct {
	*localDestroyAllCommand
	manifest *model.Manifest
}

func newLocalDestroyer(
	manifest *model.Manifest,
	destroyerAll *localDestroyAllCommand,
) *localDestroyCommand {
	return &localDestroyCommand{
		destroyerAll,
		manifest,
	}
}

func (ld *localDestroyCommand) destroy(ctx context.Context, opts *Options) error {

	if opts.RunInRemote {
		if ld.manifest.Destroy != nil && ld.manifest.Destroy.Image == "" {
			ld.manifest.Destroy.Image = constants.OktetoPipelineRunnerImage
		}
	}

	err := ld.runDestroy(ctx, opts)
	if err == nil {
		oktetoLog.Success("Development environment '%s' successfully destroyed", opts.Name)
	}
	analytics.TrackDestroy(err == nil, opts.DestroyAll)

	return err
}

func (ld *localDestroyCommand) runDestroy(ctx context.Context, opts *Options) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get the current working directory: %w", err)
	}
	if opts.Name == "" {
		if ld.manifest.Name != "" {
			opts.Name = ld.manifest.Name
		} else {
			opts.Name = utils.InferName(cwd)
		}

	}
	err = ld.manifest.ExpandEnvVars()
	if err != nil {
		return err
	}

	for _, variable := range opts.Variables {
		value := strings.SplitN(variable, "=", 2)[1]
		if strings.TrimSpace(value) != "" {
			oktetoLog.AddMaskedWord(value)
		}
	}
	oktetoLog.EnableMasking()

	namespace := opts.Namespace
	if namespace == "" {
		namespace = okteto.Context().Namespace
	}

	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Destroying...")

	data := &pipeline.CfgData{
		Name:      opts.Name,
		Namespace: namespace,
		Status:    pipeline.DestroyingStatus,
		Filename:  opts.ManifestPathFlag,
	}

	cfg, err := ld.configMapHandler.translateConfigMapAndDeploy(ctx, data)
	if err != nil {
		return err
	}

	if ld.manifest.Context == "" {
		ld.manifest.Context = okteto.Context().Name
	}
	if ld.manifest.Namespace == okteto.Context().Namespace {
		ld.manifest.Namespace = okteto.Context().Namespace
	}
	os.Setenv(constants.OktetoNameEnvVar, opts.Name)

	if opts.DestroyDependencies {
		for depName, depInfo := range ld.manifest.Dependencies {
			oktetoLog.SetStage(fmt.Sprintf("Destroying dependency '%s'", depName))

			namespace := okteto.Context().Namespace
			if depInfo.Namespace != "" {
				namespace = depInfo.Namespace
			}

			destOpts := &pipelineCMD.DestroyOptions{
				Name:           depName,
				DestroyVolumes: opts.DestroyVolumes,
				Namespace:      namespace,
			}
			pipelineCmd, err := pipelineCMD.NewCommand()
			if err != nil {
				if err := ld.configMapHandler.setErrorStatus(ctx, cfg, data, err); err != nil {
					return err
				}
				return err
			}
			if err := pipelineCmd.ExecuteDestroyPipeline(ctx, destOpts); err != nil {
				if err := ld.configMapHandler.setErrorStatus(ctx, cfg, data, err); err != nil {
					return err
				}
				return err
			}
		}
		oktetoLog.SetStage("")
	}

	var commandErr error
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)

	go func() {
		for _, command := range ld.manifest.Destroy.Commands {
			oktetoLog.Information("Running '%s'", command.Name)
			oktetoLog.SetStage(command.Name)
			if err := ld.executor.Execute(command, opts.Variables); err != nil {
				err = fmt.Errorf("error executing command '%s': %s", command.Name, err.Error())
				if !opts.ForceDestroy {
					if err := ld.configMapHandler.setErrorStatus(ctx, cfg, data, err); err != nil {
						exit <- err
						return
					}
					exit <- err
					return
				}

				// Store the error to return if the force destroy option is set
				commandErr = err
			}
		}
		exit <- nil
	}()
	select {
	case <-stop:
		oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
		errStop := "interrupt signal received"
		ld.executor.CleanUp(errors.New(errStop))
		if err := ld.configMapHandler.setErrorStatus(ctx, cfg, data, fmt.Errorf(errStop)); err != nil {
			return err
		}
		return oktetoErrors.ErrIntSig
	case err := <-exit:
		if err != nil {
			oktetoLog.Infof("exit signal received due to error: %s", err)
			return err
		}
	}
	oktetoLog.SetStage("")
	oktetoLog.DisableMasking()

	oktetoLog.Spinner(fmt.Sprintf("Destroying development environment '%s'...", opts.Name))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	deployedByLs, err := labels.NewRequirement(
		model.DeployedByLabel,
		selection.Equals,
		[]string{format.ResourceK8sMetaString(opts.Name)},
	)
	if err != nil {
		if err := ld.configMapHandler.setErrorStatus(ctx, cfg, data, err); err != nil {
			return err
		}
		return err
	}
	deployedBySelector := labels.NewSelector().Add(*deployedByLs).String()
	deleteOpts := namespaces.DeleteAllOptions{
		LabelSelector:  deployedBySelector,
		IncludeVolumes: opts.DestroyVolumes,
	}

	oktetoLog.SetStage("Destroying volumes")
	if err := ld.nsDestroyer.DestroySFSVolumes(ctx, opts.Namespace, deleteOpts); err != nil {
		if err := ld.configMapHandler.setErrorStatus(ctx, cfg, data, err); err != nil {
			return err
		}
		return err
	}

	oktetoLog.SetStage("Destroying Helm release")
	if err := ld.destroyHelmReleasesIfPresent(ctx, opts, deployedBySelector); err != nil {
		if !opts.ForceDestroy {
			if err := ld.configMapHandler.setErrorStatus(ctx, cfg, data, err); err != nil {
				return err
			}
			return err
		}
	}

	oktetoLog.Debugf("destroying resources with deployed-by label '%s'", deployedBySelector)
	oktetoLog.SetStage(fmt.Sprintf("Destroying by label '%s'", deployedBySelector))
	if err := ld.nsDestroyer.DestroyWithLabel(ctx, opts.Namespace, deleteOpts); err != nil {
		oktetoLog.Infof("could not delete all the resources: %s", err)
		if err := ld.configMapHandler.setErrorStatus(ctx, cfg, data, err); err != nil {
			return err
		}
		return err
	}

	oktetoLog.SetStage("Destroying configmap")

	if err := ld.configMapHandler.destroyConfigMap(ctx, cfg, namespace); err != nil {
		return err
	}

	return commandErr
}

func (dc *localDestroyCommand) destroyHelmReleasesIfPresent(ctx context.Context, opts *Options, labelSelector string) error {
	sList, err := dc.secrets.List(ctx, opts.Namespace, labelSelector)
	if err != nil {
		return err
	}

	oktetoLog.Debugf("checking if application installed something with helm")
	helmReleases := map[string]bool{}
	for _, s := range sList {
		if s.Type == model.HelmSecretType && s.Labels[ownerLabel] == helmOwner {
			helmReleaseName, ok := s.Labels[nameLabel]
			if !ok {
				continue
			}

			helmReleases[helmReleaseName] = true
		}
	}

	// If the application to be destroyed was deployed with helm, we try to uninstall it to avoid to leave orphan release resources
	for releaseName := range helmReleases {
		oktetoLog.Debugf("uninstalling helm release '%s'", releaseName)
		cmd := fmt.Sprintf(helmUninstallCommand, releaseName)
		cmdInfo := model.DeployCommand{Command: cmd, Name: cmd}
		oktetoLog.Information("Running '%s'", cmdInfo.Name)
		if err := dc.executor.Execute(cmdInfo, opts.Variables); err != nil {
			oktetoLog.Infof("could not uninstall helm release '%s': %s", releaseName, err)
			if !opts.ForceDestroy {
				return err
			}
		}
	}

	return nil
}
