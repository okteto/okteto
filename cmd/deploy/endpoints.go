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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
)

// EndpointsOptions defines the options to get the endpoints
type EndpointsOptions struct {
	Name         string
	ManifestPath string
	Output       string
	Namespace    string
	K8sContext   string
}

// Endpoints deploys the okteto manifest
func Endpoints(ctx context.Context) *cobra.Command {
	options := &EndpointsOptions{}
	cmd := &cobra.Command{
		Use:   "endpoints",
		Short: "Show endpoints for an environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			if options.ManifestPath != "" {
				workdir := utils.GetWorkdirFromManifestPath(options.ManifestPath)
				if err := os.Chdir(workdir); err != nil {
					return err
				}
				options.ManifestPath = utils.GetManifestPathFromWorkdir(options.ManifestPath, workdir)
			}

			ctxResource, err := utils.LoadManifestContext(options.ManifestPath)
			if err != nil {
				if oktetoErrors.IsNotExist(err) {
					ctxResource = &model.ContextResource{}
				}
			}

			if err := ctxResource.UpdateNamespace(options.Namespace); err != nil {
				return err
			}

			if err := ctxResource.UpdateContext(options.K8sContext); err != nil {
				return err
			}

			ctxOptions := &contextCMD.ContextOptions{
				Context:   ctxResource.Context,
				Namespace: ctxResource.Namespace,
			}
			if options.Output == "" {
				ctxOptions.Show = true
			}
			if err := contextCMD.NewContextCommand().Run(ctx, ctxOptions); err != nil {
				return err
			}
			c := &DeployCommand{
				GetManifest:       model.GetManifestV2,
				K8sClientProvider: okteto.NewK8sClientProvider(),
			}
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get the current working directory: %w", err)
			}

			if options.Name == "" {
				manifest, err := c.GetManifest(options.ManifestPath)
				if err != nil {
					return err
				}
				if manifest.Name != "" {
					options.Name = manifest.Name
				} else {
					options.Name = utils.InferName(cwd)
				}
				if options.Namespace == "" {
					options.Namespace = manifest.Namespace
				}
			}
			if options.Namespace == "" {
				options.Namespace = okteto.Context().Namespace
			}

			if err := validateOutput(options.Output); err != nil {
				return err
			}
			return c.showEndpoints(ctx, options)
		},
	}
	cmd.Flags().StringVar(&options.Name, "name", "", "development environment name")
	cmd.Flags().StringVarP(&options.ManifestPath, "file", "f", "", "path to the okteto manifest file")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "overwrites the namespace where the development environment is deployed")
	cmd.Flags().StringVarP(&options.K8sContext, "context", "c", "", "context where the development environment is deployed")

	cmd.Flags().StringVarP(&options.Output, "output", "o", "", "output format. One of: ['json', 'md']")

	return cmd
}

func validateOutput(output string) error {
	switch output {
	case "", "json", "md":
		return nil
	default:
		return fmt.Errorf("output format is not accepted. Value must be one of: ['json', 'md']")
	}
}

func (dc *DeployCommand) getEndpoints(ctx context.Context, opts *EndpointsOptions) ([]string, error) {
	spinner := utils.NewSpinner("Retrieving endpoints...")
	if opts.Output == "" {
		spinner.Start()
		defer spinner.Stop()
	}

	labelSelector := fmt.Sprintf("%s=%s", model.DeployedByLabel, opts.Name)
	iClient, err := dc.K8sClientProvider.GetIngressClient(ctx)
	if err != nil {
		return nil, err
	}
	eps, err := iClient.GetEndpointsBySelector(ctx, opts.Namespace, labelSelector)
	if err != nil {
		return nil, err
	}
	spinner.Stop()
	if len(eps) > 0 {
		sort.Slice(eps, func(i, j int) bool {
			return len(eps[i]) < len(eps[j])
		})
	}
	return eps, nil
}

func (dc *DeployCommand) showEndpoints(ctx context.Context, opts *EndpointsOptions) error {
	eps, err := dc.getEndpoints(ctx, opts)
	if err != nil {
		return err
	}

	switch opts.Output {
	case "json":
		bytes, err := json.MarshalIndent(eps, "", "  ")
		if err != nil {
			return err
		}
		oktetoLog.Println(string(bytes))
	case "md":
		if len(eps) == 0 {
			oktetoLog.Printf("There are no available endpoints for '%s'\n", opts.Name)
		} else {
			oktetoLog.Printf("Available endpoints:\n")
			for _, e := range eps {
				oktetoLog.Printf("\n - [%s](%s)\n", e, e)
			}
		}
	default:
		if len(eps) == 0 {
			oktetoLog.Printf("There are no available endpoints for '%s'\n", opts.Name)
		} else {
			oktetoLog.Information("Endpoints available:")
			oktetoLog.Printf("  - %s\n", strings.Join(eps, "\n  - "))
		}
	}
	return nil
}
