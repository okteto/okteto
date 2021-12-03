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

package destroy

import (
	"context"
	"fmt"
	"os"

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
	DestroyWithLabel(ctx context.Context, ns string, opts namespaces.DeleteAllOptions) error
	DestroySFSVolumes(ctx context.Context, ns string, opts namespaces.DeleteAllOptions) error
}

type secretHandler interface {
	List(ctx context.Context, ns, labelSelector string) ([]v1.Secret, error)
}

// Options destroy commands options
type Options struct {
	ManifestPath   string
	Name           string
	Variables      []string
	Namespace      string
	DestroyVolumes bool
	ForceDestroy   bool
	OutputMode     string
	K8sContext     string
}

type destroyCommand struct {
	getManifest func(ctx context.Context, cwd string, opts contextCMD.ManifestOptions) (*model.Manifest, error)

	executor    utils.ManifestExecutor
	nsDestroyer destroyer
	secrets     secretHandler
}

// Destroy destroys the dev application defined by the manifest
func Destroy(ctx context.Context) *cobra.Command {
	options := &Options{
		Variables: []string{},
	}

	cmd := &cobra.Command{
		Use:    "destroy",
		Short:  `Destroy everything created by "okteto deploy" command`,
		Long:   `Destroy everything created by "okteto deploy" command. You can also include a "destroy" section in your Okteto manifest with a list of commands to be executed when the application is destroyed`,
		Args:   utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#version"),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctxOptions := &contextCMD.ContextOptions{
				Show: true,
			}
			if err := contextCMD.Run(ctx, ctxOptions); err != nil {
				return err
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get the current working directory: %w", err)
			}

			if options.Name == "" {
				options.Name = utils.InferApplicationName(cwd)
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

			if options.Namespace == "" {
				options.Namespace = okteto.Context().Namespace
			}

			c := &destroyCommand{
				getManifest: contextCMD.GetManifest,

				executor:    utils.NewExecutor(options.OutputMode),
				nsDestroyer: namespaces.NewNamespace(dynClient, discClient, cfg, k8sClient),
				secrets:     secrets.NewSecrets(k8sClient),
			}
			return c.runDestroy(ctx, cwd, options)
		},
	}

	cmd.Flags().StringVar(&options.Name, "name", "", "application name")
	cmd.Flags().StringVarP(&options.ManifestPath, "file", "f", "", "path to the manifest file")
	cmd.Flags().StringVarP(&options.OutputMode, "output", "o", "tty", "show plain/tty deploy output")
	cmd.Flags().BoolVarP(&options.DestroyVolumes, "volumes", "v", false, "remove persistent volumes")
	cmd.Flags().BoolVar(&options.ForceDestroy, "force-destroy", false, "forces the application destroy even if there is an error executing the custom destroy commands defined in the manifest")
	cmd.Flags().StringVar(&options.Namespace, "namespace", "", "application name")
	cmd.Flags().StringVar(&options.K8sContext, "context", "", "k8s context")

	return cmd
}

func (dc *destroyCommand) runDestroy(ctx context.Context, cwd string, opts *Options) error {
	// Read manifest file with the commands to be executed
	manifest, err := dc.getManifest(ctx, cwd, contextCMD.ManifestOptions{Name: opts.Name, Filename: opts.ManifestPath, Namespace: opts.Namespace, K8sContext: opts.K8sContext})
	if err != nil {
		// Log error message but application can still be deleted
		log.Infof("could not find manifest file to be executed: %s", err)
		manifest = &model.Manifest{
			Destroy: []string{},
		}
	}

	var commandErr error
	for _, command := range manifest.Destroy {
		if err := dc.executor.Execute(command, opts.Variables); err != nil {
			log.Infof("error executing command '%s': %s", command, err.Error())
			if !opts.ForceDestroy {
				return err
			}

			// Store the error to return if the force destroy option is set
			commandErr = err
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
	deleteOpts := namespaces.DeleteAllOptions{
		LabelSelector:  deployedBySelector,
		IncludeVolumes: opts.DestroyVolumes,
	}

	if err := dc.nsDestroyer.DestroySFSVolumes(ctx, opts.Namespace, deleteOpts); err != nil {
		return err
	}

	if err := dc.destroyHelmReleasesIfPresent(ctx, opts, deployedBySelector); err != nil {
		if !opts.ForceDestroy {
			return err
		}
	}

	log.Debugf("destroying resources with deployed-by label '%s'", deployedBySelector)
	if err := dc.nsDestroyer.DestroyWithLabel(ctx, opts.Namespace, deleteOpts); err != nil {
		log.Infof("could not delete all the resources: %s", err)
		return err
	}

	return commandErr
}

func (dc *destroyCommand) destroyHelmReleasesIfPresent(ctx context.Context, opts *Options, labelSelector string) error {
	sList, err := dc.secrets.List(ctx, opts.Namespace, labelSelector)
	if err != nil {
		return err
	}

	log.Debugf("checking if application installed something with helm")
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
		log.Debugf("uninstalling helm release %s", releaseName)
		cmd := fmt.Sprintf(helmUninstallCommand, releaseName)
		if err := dc.executor.Execute(cmd, opts.Variables); err != nil {
			log.Infof("could not uninstall helm release '%s': %s", releaseName, err)
			if !opts.ForceDestroy {
				return err
			}
		}
	}

	return nil
}
