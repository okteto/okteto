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

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/constants"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	apiv1 "k8s.io/api/core/v1"
)

// configMapHandler defines the different functions to run okteto inside an okteto deploy
// or an okteto destroy directly
type configMapHandler interface {
	translateConfigMapAndDeploy(context.Context, *pipeline.CfgData) (*apiv1.ConfigMap, error)
	updateConfigMap(context.Context, *apiv1.ConfigMap, *pipeline.CfgData, error) error
	updateEnvsFromCommands(context.Context, string, string, []string) error
	getConfigmapVariablesEncoded(ctx context.Context, name, namespace string) (string, error)
}

// deployInsideDeployConfigMapHandler is the runner used when the okteto is executed
// inside an okteto deploy command
type deployInsideDeployConfigMapHandler struct{}

func newDeployInsideDeployConfigMapHandler() *deployInsideDeployConfigMapHandler {
	return &deployInsideDeployConfigMapHandler{}
}

// oktetoDefaultConfigMapHandler is the runner used when the okteto is executed
// directly
type defaultConfigMapHandler struct {
	k8sClientProvider okteto.K8sClientProvider
}

func newDefaultConfigMapHandler(provider okteto.K8sClientProvider) *defaultConfigMapHandler {
	return &defaultConfigMapHandler{
		k8sClientProvider: provider,
	}
}

func NewConfigmapHandler(provider okteto.K8sClientProvider) configMapHandler {
	if utils.LoadBoolean(constants.OKtetoDeployRemote) {
		return newDeployInsideDeployConfigMapHandler()
	}
	return newDefaultConfigMapHandler(provider)
}

func (ch *defaultConfigMapHandler) translateConfigMapAndDeploy(ctx context.Context, data *pipeline.CfgData) (*apiv1.ConfigMap, error) {
	c, _, err := ch.k8sClientProvider.Provide(okteto.Context().Cfg)
	if err != nil {
		return nil, err
	}
	return pipeline.TranslateConfigMapAndDeploy(ctx, data, c)
}

func (ch *defaultConfigMapHandler) getConfigmapVariablesEncoded(ctx context.Context, name, namespace string) (string, error) {
	c, _, err := ch.k8sClientProvider.Provide(okteto.Context().Cfg)
	if err != nil {
		return "", err
	}
	return pipeline.GetConfigmapVariablesEncoded(ctx, name, namespace, c)
}

func (ch *defaultConfigMapHandler) updateConfigMap(ctx context.Context, cfg *apiv1.ConfigMap, data *pipeline.CfgData, errMain error) error {
	c, _, err := ch.k8sClientProvider.Provide(okteto.Context().Cfg)
	if err != nil {
		return err
	}
	if errMain != nil {
		oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, errMain.Error())
		data.Status = pipeline.ErrorStatus
	}
	if err := pipeline.UpdateConfigMap(ctx, cfg, data, c); err != nil {
		return err
	}
	return errMain
}

// updateEnvsFromCommands update config map by adding envs generated in OKTETO_ENV as data fields
func (h *defaultConfigMapHandler) updateEnvsFromCommands(ctx context.Context, name, namespace string, envs []string) error {
	c, _, err := h.k8sClientProvider.Provide(okteto.Context().Cfg)
	if err != nil {
		return err
	}

	if err := pipeline.UpdateEnvs(ctx, name, namespace, envs, c); err != nil {
		return err
	}
	return nil
}

// translateConfigMapAndDeploy with the receiver deployInsideDeployConfigMapHandler doesn't do anything
// because we have to  control the cfmap in the main execution. If both handled the configmap we will be
// overwritten the cfmap and leave it in a inconsistent status
func (*deployInsideDeployConfigMapHandler) translateConfigMapAndDeploy(_ context.Context, _ *pipeline.CfgData) (*apiv1.ConfigMap, error) {
	return nil, nil
}

func (*deployInsideDeployConfigMapHandler) getConfigmapVariablesEncoded(_ context.Context, _, _ string) (string, error) {
	return "", nil
}

// updateConfigMap with the receiver deployInsideDeployConfigMapHandler doesn't do anything
// because we have to  control the cfmap in the main execution. If both handled the configmap we will be
// overwritten the cfmap and leave it in a inconsistent status
func (*deployInsideDeployConfigMapHandler) updateConfigMap(_ context.Context, _ *apiv1.ConfigMap, _ *pipeline.CfgData, err error) error {
	return nil
}

// updateEnvs with the receiver deployInsideDeployConfigMapHandler doesn't do anything
// because we have to  control the cfmap in the main execution. If both handled the configmap we will be
// overwritten the cfmap and leave it in a inconsistent status
func (*deployInsideDeployConfigMapHandler) updateEnvsFromCommands(_ context.Context, _ string, _ string, _ []string) error {
	return nil
}
