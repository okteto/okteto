// Copyright 2023 The Okteto Authors
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
	"github.com/okteto/okteto/pkg/devenvironment"
	"github.com/okteto/okteto/pkg/endpoints"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/k8s/ingresses"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
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

type endpointGetterInterface interface {
	List(ctx context.Context, ns string, devName string) ([]string, error)
}

type endpointControlInterface interface {
	List(ctx context.Context, opts *EndpointsOptions, devName string) ([]string, error)
}

type k8sIngressClientProvider interface {
	GetIngressClient() (*ingresses.Client, error)
}

type EndpointGetter struct {
	GetManifest     func(path string) (*model.Manifest, error)
	endpointControl endpointControlInterface
}

func NewEndpointGetter(k8sLogger *io.K8sLogger) (EndpointGetter, error) {
	var endpointControl endpointControlInterface
	if okteto.GetContext().IsOkteto {
		c, err := okteto.NewOktetoClient()
		if err != nil {
			return EndpointGetter{}, err
		}
		endpointControl = NewEndpointGetterWithOktetoAPI(c)
	} else {
		endpointControl = NewEndpointGetterInStandaloneMode(k8sLogger)
	}

	return EndpointGetter{
		GetManifest:     model.GetManifestV2,
		endpointControl: endpointControl,
	}, nil

}

// Endpoints deploys the okteto manifest
func Endpoints(ctx context.Context, k8sLogger *io.K8sLogger) *cobra.Command {
	options := &EndpointsOptions{}
	cmd := &cobra.Command{
		Use:   "endpoints",
		Short: "Show endpoints for an environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			if options.ManifestPath != "" {
				workdir := model.GetWorkdirFromManifestPath(options.ManifestPath)
				if err := os.Chdir(workdir); err != nil {
					return err
				}
				options.ManifestPath = model.GetManifestPathFromWorkdir(options.ManifestPath, workdir)
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

			ctxOptions := &contextCMD.Options{
				Context:   ctxResource.Context,
				Namespace: ctxResource.Namespace,
			}
			if options.Output == "" {
				ctxOptions.Show = true
			}
			if err := contextCMD.NewContextCommand().Run(ctx, ctxOptions); err != nil {
				return err
			}

			eg, err := NewEndpointGetter(k8sLogger)
			if err != nil {
				return err
			}
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get the current working directory: %w", err)
			}

			if options.Name == "" {
				manifest, err := eg.GetManifest(options.ManifestPath)
				if err != nil {
					return err
				}
				if manifest.Name != "" {
					options.Name = manifest.Name
				} else {
					c, _, err := okteto.NewK8sClientProviderWithLogger(k8sLogger).Provide(okteto.GetContext().Cfg)
					if err != nil {
						return err
					}
					inferer := devenvironment.NewNameInferer(c)
					options.Name = inferer.InferName(ctx, cwd, okteto.GetContext().Namespace, options.ManifestPath)
				}
				if options.Namespace == "" {
					options.Namespace = manifest.Namespace
				}
			}
			if options.Namespace == "" {
				options.Namespace = okteto.GetContext().Namespace
			}

			if err := validateOutput(options.Output); err != nil {
				return err
			}
			return eg.showEndpoints(ctx, options)
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

func (eg *EndpointGetter) getEndpoints(ctx context.Context, opts *EndpointsOptions) ([]string, error) {
	if opts.Output == "" {
		oktetoLog.Spinner("Retrieving endpoints...")
		oktetoLog.StartSpinner()
		defer oktetoLog.StopSpinner()
	}

	sanitizedName := format.ResourceK8sMetaString(opts.Name)

	eps, err := eg.endpointControl.List(ctx, opts, sanitizedName)
	if err != nil {
		return nil, err
	}

	if len(eps) > 0 {
		sort.Slice(eps, func(i, j int) bool {
			return len(eps[i]) < len(eps[j])
		})
	}
	return eps, nil
}

type endpointGetterWithOktetoAPI struct {
	endpointControl endpointGetterInterface
}

func NewEndpointGetterWithOktetoAPI(c *okteto.Client) *endpointGetterWithOktetoAPI {
	return &endpointGetterWithOktetoAPI{
		endpointControl: endpoints.NewEndpointControl(c),
	}
}

func (eg *endpointGetterWithOktetoAPI) List(ctx context.Context, opts *EndpointsOptions, devName string) ([]string, error) {
	return eg.endpointControl.List(ctx, opts.Namespace, devName)
}

type endpointGetterInStandaloneMode struct {
	k8sClientProvider k8sIngressClientProvider
	getEndpoints      func(context.Context, *EndpointsOptions, string, k8sIngressClientProvider) ([]string, error)
}

func NewEndpointGetterInStandaloneMode(k8sLogger *io.K8sLogger) *endpointGetterInStandaloneMode {
	return &endpointGetterInStandaloneMode{
		k8sClientProvider: okteto.NewK8sClientProviderWithLogger(k8sLogger),
		getEndpoints:      getEndpointsStandaloneMode,
	}
}

func (eg *endpointGetterInStandaloneMode) List(ctx context.Context, opts *EndpointsOptions, devName string) ([]string, error) {
	labelSelector := fmt.Sprintf("%s=%s", model.DeployedByLabel, devName)
	eps, err := eg.getEndpoints(ctx, opts, labelSelector, eg.k8sClientProvider)
	if err != nil {
		return nil, err
	}

	return eps, nil
}

func getEndpointsStandaloneMode(ctx context.Context, opts *EndpointsOptions, labelSelector string, k8sClientProvider k8sIngressClientProvider) ([]string, error) {
	var eps []string
	iClient, err := k8sClientProvider.GetIngressClient()
	if err != nil {
		return nil, err
	}
	eps, err = iClient.GetEndpointsBySelector(ctx, opts.Namespace, labelSelector)
	if err != nil {
		return nil, err
	}
	return eps, nil
}

func (dc *EndpointGetter) showEndpoints(ctx context.Context, opts *EndpointsOptions) error {
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
			oktetoLog.Information("There are no available endpoints for '%s'.\n    Follow this link to know more about how to create public endpoints for your application:\n    https://www.okteto.com/docs/core/ingress/automatic-ssl", opts.Name)
		} else {
			oktetoLog.Information("Endpoints available:")
			oktetoLog.Printf("  - %s\n", strings.Join(eps, "\n  - "))
		}
	}
	return nil
}
