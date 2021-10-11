package destroy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/k8s/secrets"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

const (
	ownerLabel           = "owner"
	nameLabel            = "name"
	helmOwner            = "helm"
	typeField            = "type"
	helmUninstallCommand = "helm uninstall %s"
)

type destroyer interface {
	DestroyWithLabel(ctx context.Context, ns, labelSelector string) error
}

type secretHandler interface {
	List(ctx context.Context, ns, labelSelector string) ([]v1.Secret, error)
}

// Options destroy commands options
type Options struct {
	ManifestPath string
	Name         string
	Variables    []string
	Namespace    string
}

type destroyCommand struct {
	getManifest func(cwd, name, filename string) (*utils.Manifest, error)

	executor    utils.ManifestExecutor
	nsDestroyer destroyer
	secrets     secretHandler
}

// Destroy destroys the dev application defined by the manifest
func Destroy(ctx context.Context) *cobra.Command {
	options := &Options{}

	cmd := &cobra.Command{
		Use:    "destroy",
		Short:  "Executes the list of commands specified in the Okteto manifest to destroy the application",
		Args:   utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#version"),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := contextCMD.Init(ctx); err != nil {
				return err
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get the current working directory: %w", err)
			}

			if options.Name == "" {
				options.Name = getName(cwd)
				if err != nil {
					return fmt.Errorf("could not infer environment name")
				}
			}

			dynClient, _, err := okteto.GetDynamicClient()
			if err != nil {
				return err
			}
			discClient, _, err := okteto.GetDiscoveryClient()
			if err != nil {
				return err
			}
			k8sClient, cfg, err := okteto.GetK8sClient()
			if err != nil {
				return err
			}

			options.Namespace = okteto.Context().Namespace
			c := &destroyCommand{
				getManifest: utils.GetManifest,

				executor:    utils.NewExecutor(),
				nsDestroyer: namespaces.NewNamespace(dynClient, discClient, cfg),
				secrets:     secrets.NewSecrets(k8sClient),
			}
			return c.runDestroy(ctx, cwd, options)
		},
	}

	cmd.Flags().StringVarP(&options.Name, "name", "a", "", "application name")
	cmd.Flags().StringVarP(&options.ManifestPath, "filename", "f", "", "path to the manifest file")
	cmd.Flags().StringArrayVarP(&options.Variables, "var", "v", []string{}, "set a variable (can be set more than once)")

	return cmd
}

func (dc *destroyCommand) runDestroy(ctx context.Context, cwd string, opts *Options) error {
	// Read manifest file with the commands to be executed
	manifest, err := dc.getManifest(cwd, opts.Name, opts.ManifestPath)
	if err != nil {
		// Log error message but application can still be deleted
		log.Errorf("could not find manifest file to be executed: %s", err)
	}

	var commandErr error
	for _, command := range manifest.Destroy {
		if err := dc.executor.Execute(command, opts.Variables); err != nil {
			log.Errorf("error executing command '%s': %s", command, err.Error())
			commandErr = err
			break
		}
	}

	deployedByLs, err := labels.NewRequirement(
		model.DeployedByLabel,
		selection.Equals,
		[]string{opts.Name},
	)
	if err != nil {
		return err
	}
	deployedBySelector := labels.NewSelector().Add(*deployedByLs).String()

	sList, err := dc.secrets.List(ctx, opts.Namespace, deployedBySelector)
	if err != nil {
		return err
	}

	log.Debugf("checking if application installed something with helm")
	var helmReleaseName string
	for _, s := range sList {
		if s.Type == model.HelmSecretType && s.Labels[ownerLabel] == helmOwner {
			if helmReleaseName = s.Labels[nameLabel]; helmReleaseName != "" {
				break
			}
		}
	}

	// If the application to be destroyed was deployed with helm, we try to uninstall it to avoid to leave orphan release resources
	if helmReleaseName != "" {
		log.Debugf("uninstalling helm release %s", helmReleaseName)
		cmd := fmt.Sprintf(helmUninstallCommand, helmReleaseName)
		if err := dc.executor.Execute(cmd, opts.Variables); err != nil {
			log.Errorf("could not uninstall helm release %s: %s", helmReleaseName, err)
		}
	}

	log.Debugf("destroying resources with deployed-by label %s", deployedBySelector)
	if err := dc.nsDestroyer.DestroyWithLabel(ctx, opts.Namespace, deployedBySelector); err != nil {
		log.Errorf("could not delete all the resources: %s", err)
		return err
	}

	return commandErr
}

// This will probably will be moved to a common place when we implement the full dev spec
func getName(cwd string) string {
	repo, err := model.GetRepositoryURL(cwd)
	if err != nil {
		log.Info("inferring name from folder")
		return filepath.Base(cwd)
	}

	log.Info("inferring name from git repository URL")
	return model.TranslateURLToName(repo)
}
