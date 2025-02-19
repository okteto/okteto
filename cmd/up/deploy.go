// Copyright 2025 The Okteto Authors
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

package up

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/okteto/okteto/cmd/deploy"
	pipelineCMD "github.com/okteto/okteto/cmd/pipeline"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/afero"
	"k8s.io/client-go/kubernetes"
)

const (
	// oktetoAutoDeployEnvVar if set the application will be deployed while running okteto up
	oktetoAutoDeployEnvVar = "OKTETO_AUTODEPLOY"
)

// devEnvDeployerManager deploys the dev environment
type devEnvDeployerManager struct {
	isDevEnvDeployed func(ctx context.Context, name, namespace string, c kubernetes.Interface) bool

	k8sClientProvider okteto.K8sClientProvider
	ioCtrl            *io.Controller
	getDeployer       func(deployParams) (deployer, error)
}

type deployer interface {
	Run(ctx context.Context, opts *deploy.Options) error
	TrackDeploy(manifest *model.Manifest, runInRemoteFlag bool, startTime time.Time, err error)
}

type deployParams struct {
	deployFlag                     bool
	okCtx                          *okteto.Context
	devenvName, ns                 string
	manifestPathFlag, manifestPath string
	manifest                       *model.Manifest
}

// NewDevEnvDeployerManager creates a new DevEnvDeployer
func NewDevEnvDeployerManager(up *upContext, okCtx *okteto.Context, ioCtrl *io.Controller, k8sLogger *io.K8sLogger) *devEnvDeployerManager {
	return &devEnvDeployerManager{
		ioCtrl:            ioCtrl,
		k8sClientProvider: up.K8sClientProvider,
		isDevEnvDeployed:  pipeline.IsDeployed,
		getDeployer: func(params deployParams) (deployer, error) {
			k8sProvider := okteto.NewK8sClientProviderWithLogger(k8sLogger)
			pc, err := pipelineCMD.NewCommand()
			if err != nil {
				return nil, err
			}
			c := &deploy.Command{
				// reuse the manifest we already have in the upContext so we don't open a file again
				GetManifest: func(string, afero.Fs) (*model.Manifest, error) {
					return params.manifest, nil
				},
				GetDeployer:       deploy.GetDeployer,
				K8sClientProvider: k8sProvider,
				Builder:           up.builder,
				Fs:                up.Fs,
				CfgMapHandler:     deploy.NewConfigmapHandler(k8sProvider, k8sLogger),
				PipelineCMD:       pc,
				DeployWaiter:      deploy.NewDeployWaiter(k8sProvider, k8sLogger),
				EndpointGetter:    deploy.NewEndpointGetter,
				AnalyticsTracker:  up.analyticsTracker,
				IoCtrl:            ioCtrl,
			}
			return c, nil
		},
	}
}

// DeployIfNeeded deploys the app if it's not already deployed or if the user has set the auto deploy env var or the --deploy flag
func (dd *devEnvDeployerManager) DeployIfNeeded(ctx context.Context, params deployParams, analyticsMeta *analytics.UpMetricsMetadata) error {
	if !params.okCtx.IsOkteto {
		dd.ioCtrl.Logger().Infof("Deploy is skipped because is not okteto context")
		return nil
	}
	k8sClient, _, err := dd.k8sClientProvider.Provide(params.okCtx.Cfg)
	if err != nil {
		return err
	}

	mustDeploy := params.deployFlag
	if env.LoadBoolean(oktetoAutoDeployEnvVar) {
		mustDeploy = true
	}

	if mustDeploy || !dd.isDevEnvDeployed(ctx, params.devenvName, params.ns, k8sClient) {
		deployer, err := dd.getDeployer(params)
		if err != nil {
			dd.ioCtrl.Logger().Infof("failed to create deployer: %s", err)
			return fmt.Errorf("failed to create deployer: %w", err)
		}

		deployOpts := &deploy.Options{
			Name:             params.devenvName,
			Namespace:        params.ns,
			ManifestPathFlag: params.manifestPathFlag,
			ManifestPath:     params.manifestPath,
			Timeout:          5 * time.Minute,
			NoBuild:          false,
		}
		startTime := time.Now()
		err = deployer.Run(ctx, deployOpts)
		go analyticsMeta.HasRunDeploy()
		deployer.TrackDeploy(params.manifest, deploy.ShouldRunInRemote(deployOpts), startTime, err)
		// only allow error.ErrManifestFoundButNoDeployAndDependenciesCommands to go forward - autocreate property will deploy the app
		if err != nil && !errors.Is(err, oktetoErrors.ErrManifestFoundButNoDeployAndDependenciesCommands) {
			return err
		}

	} else {
		oktetoLog.Information("'%s' was already deployed. To redeploy run 'okteto deploy' or 'okteto up --deploy'", params.devenvName)
	}
	return nil
}
