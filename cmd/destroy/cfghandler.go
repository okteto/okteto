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

package destroy

import (
	"context"

	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/vars"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// configMapHandler defines the different functions to run okteto inside an okteto deploy
// or an okteto destroy directly
type configMapHandler interface {
	translateConfigMapAndDeploy(context.Context, *pipeline.CfgData) (*apiv1.ConfigMap, error)
	destroyConfigMap(context.Context, *apiv1.ConfigMap, string) error
	setErrorStatus(context.Context, *apiv1.ConfigMap, *pipeline.CfgData, error) error
	getConfigmapVariablesEncoded(ctx context.Context, name, namespace string) (string, error)
}

// oktetoDefaultConfigMapHandler is the runner used when the okteto is executed
// directly
type defaultConfigMapHandler struct {
	k8sClient  kubernetes.Interface
	varManager *vars.Manager
}

func newDefaultConfigMapHandler(c kubernetes.Interface, varManager *vars.Manager) *defaultConfigMapHandler {
	return &defaultConfigMapHandler{
		k8sClient:  c,
		varManager: varManager,
	}
}

func NewConfigmapHandler(c kubernetes.Interface, varManager *vars.Manager) configMapHandler {
	return newDefaultConfigMapHandler(c, varManager)
}

func (ch *defaultConfigMapHandler) translateConfigMapAndDeploy(ctx context.Context, data *pipeline.CfgData) (*apiv1.ConfigMap, error) {
	return pipeline.TranslateConfigMapAndDeploy(ctx, data, ch.k8sClient, ch.varManager)
}

func (ch *defaultConfigMapHandler) getConfigmapVariablesEncoded(ctx context.Context, name, namespace string) (string, error) {
	return pipeline.GetConfigmapVariablesEncoded(ctx, name, namespace, ch.k8sClient)
}

func (ch *defaultConfigMapHandler) destroyConfigMap(ctx context.Context, cfg *apiv1.ConfigMap, namespace string) error {
	return configmaps.Destroy(ctx, cfg.Name, namespace, ch.k8sClient)
}

func (ch *defaultConfigMapHandler) setErrorStatus(ctx context.Context, cfg *apiv1.ConfigMap, data *pipeline.CfgData, err error) error {
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Destruction failed: %s", err.Error())
	data.Status = pipeline.ErrorStatus
	return pipeline.UpdateConfigMap(ctx, cfg, data, ch.k8sClient, ch.varManager)
}
