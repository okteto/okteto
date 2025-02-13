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
	"os"
	"time"

	"github.com/okteto/okteto/cmd/deploy"
	deployCMD "github.com/okteto/okteto/cmd/deploy"
	pipelineCMD "github.com/okteto/okteto/cmd/pipeline"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/constants"
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
	// OktetoAutoDeployEnvVar if set the application will be deployed while running okteto up
	OktetoAutoDeployEnvVar = "OKTETO_AUTODEPLOY"
)

// devEnvDeployerManager deploys the dev environment
type devEnvDeployerManager struct {
	okCtx *okteto.Context

	mustDeploy bool
	devenvName string
	ns         string

	isDevEnvDeployed func(ctx context.Context, name, namespace string, c kubernetes.Interface) bool
	deployStrategy   devEnvEnvDeployStrategy

	k8sClientProvider okteto.K8sClientProvider
	ioCtrl            *io.Controller
}

// devEnvEnvDeployStrategy defines the behavior for deploying an app.
type devEnvEnvDeployStrategy interface {
	Deploy(ctx context.Context) error
}

type oktetoDevEnvDeployStrategy struct {
	manifest *model.Manifest
	okCtx    *okteto.Context

	manifestPathFlag string
	manifestPath     string
	ns               string
	devenvName       string

	builder          builderInterface
	analyticsMeta    *analytics.UpMetricsMetadata
	analyticsTracker analyticsTrackerInterface
	fs               afero.Fs

	k8sLogger         *io.K8sLogger
	k8sClientProvider okteto.K8sClientProvider
	ioCtrl            *io.Controller
}

// NewDevEnvDeployerManager creates a new DevEnvDeployer
func NewDevEnvDeployerManager(up *upContext, opts *Options, okCtx *okteto.Context, ioCtrl *io.Controller, k8sLogger *io.K8sLogger) *devEnvDeployerManager {
	mustDeploy := opts.Deploy
	if env.LoadBoolean(OktetoAutoDeployEnvVar) {
		mustDeploy = true
	}

	return &devEnvDeployerManager{
		okCtx:             okCtx,
		mustDeploy:        mustDeploy,
		devenvName:        up.Manifest.Name,
		ns:                okCtx.Namespace,
		ioCtrl:            ioCtrl,
		k8sClientProvider: up.K8sClientProvider,
		isDevEnvDeployed:  pipeline.IsDeployed,
		deployStrategy: &oktetoDevEnvDeployStrategy{
			manifest:          up.Manifest,
			okCtx:             okCtx,
			manifestPathFlag:  opts.ManifestPathFlag,
			manifestPath:      opts.ManifestPath,
			ns:                okCtx.Namespace,
			devenvName:        up.Manifest.Name,
			builder:           up.builder,
			analyticsTracker:  up.analyticsTracker,
			fs:                up.Fs,
			k8sLogger:         k8sLogger,
			k8sClientProvider: up.K8sClientProvider,
			ioCtrl:            ioCtrl,
			analyticsMeta:     up.analyticsMeta,
		},
	}
}

// DeployIfNeeded deploys the app if it's not already deployed or if the user has set the auto deploy env var or the --deploy flag
func (dd *devEnvDeployerManager) DeployIfNeeded(ctx context.Context) error {
	if !dd.okCtx.IsOkteto {
		dd.ioCtrl.Logger().Infof("Deploy is skipped because is not okteto context")
		return nil
	}
	k8sClient, _, err := dd.k8sClientProvider.Provide(dd.okCtx.Cfg)
	if err != nil {
		return err
	}

	if dd.mustDeploy || !dd.isDevEnvDeployed(ctx, dd.devenvName, dd.ns, k8sClient) {
		err := dd.deployStrategy.Deploy(ctx)
		// only allow error.ErrManifestFoundButNoDeployAndDependenciesCommands to go forward - autocreate property will deploy the app
		if err != nil && !errors.Is(err, oktetoErrors.ErrManifestFoundButNoDeployAndDependenciesCommands) {
			return err
		}

	} else {
		oktetoLog.Information("'%s' was already deployed. To redeploy run 'okteto deploy' or 'okteto up --deploy'", dd.devenvName)
	}
	return nil
}

func (od *oktetoDevEnvDeployStrategy) Deploy(ctx context.Context) error {
	k8sProvider := okteto.NewK8sClientProviderWithLogger(od.k8sLogger)
	pc, err := pipelineCMD.NewCommand()
	if err != nil {
		return err
	}
	c := &deploy.Command{
		GetManifest:       model.GetManifestV2,
		GetDeployer:       deploy.GetDeployer,
		K8sClientProvider: k8sProvider,
		Builder:           od.builder,
		Fs:                od.fs,
		CfgMapHandler:     deploy.NewConfigmapHandler(k8sProvider, od.k8sLogger),
		PipelineCMD:       pc,
		DeployWaiter:      deploy.NewDeployWaiter(k8sProvider, od.k8sLogger),
		EndpointGetter:    deploy.NewEndpointGetter,
		AnalyticsTracker:  od.analyticsTracker,
		IoCtrl:            od.ioCtrl,
	}

	deployOpts := &deploy.Options{
		Name:             od.devenvName,
		Namespace:        od.ns,
		ManifestPathFlag: od.manifestPathFlag,
		ManifestPath:     od.manifestPath,
		Timeout:          5 * time.Minute,
		NoBuild:          false,
	}
	startTime := time.Now()
	err = c.Run(ctx, deployOpts)
	od.analyticsMeta.HasRunDeploy()

	// we need to pre-calculate the value of the runInRemote flag before running the deploy command
	// in order to track the information correctly, using the same logic as the deploy command
	runInRemote := deployCMD.ShouldRunInRemote(deployOpts)

	// We keep DeprecatedOktetoCurrentDeployBelongsToPreviewEnvVar for backward compatibility in case an old version of the backend
	// is being used
	isPreview := os.Getenv(model.DeprecatedOktetoCurrentDeployBelongsToPreviewEnvVar) == "true" ||
		os.Getenv(constants.OktetoIsPreviewEnvVar) == "true"
	// tracking deploy either its been successful or not
	c.AnalyticsTracker.TrackDeploy(analytics.DeployMetadata{
		Success:                err == nil,
		IsOktetoRepo:           utils.IsOktetoRepo(),
		Duration:               time.Since(startTime),
		PipelineType:           od.manifest.Type,
		DeployType:             "automatic",
		IsPreview:              isPreview,
		HasDependenciesSection: od.manifest.HasDependenciesSection(),
		HasBuildSection:        od.manifest.HasBuildSection(),
		Err:                    err,
		IsRemote:               runInRemote,
	})
	return err
}
