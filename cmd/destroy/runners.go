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

package destroy

import (
	"context"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/k8s/configmaps"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// runnerI defines the different functions to run okteto inside an okteto deploy
// or an okteto destroy directly
type runnerI interface {
	translateConfigMapAndDeploy(context.Context, *pipeline.CfgData, kubernetes.Interface) (*apiv1.ConfigMap, error)
	destroyConfigMap(context.Context, *apiv1.ConfigMap, string, kubernetes.Interface) error
	setErrorStatus(context.Context, *apiv1.ConfigMap, *pipeline.CfgData, error, kubernetes.Interface) error
}

// oktetoInsideOktetoRunner is the runner used when the okteto is executed
// inside an okteto deploy command
type oktetoInsideOktetoRunner struct{}

// oktetoDefaultRunner is the runner used when the okteto is executed
// directly
type oktetoDefaultRunner struct{}

func newRunner() runnerI {
	if utils.LoadBoolean(model.OktetoWithinDeployCommandContextEnvVar) {
		return &oktetoInsideOktetoRunner{}
	}
	return &oktetoDefaultRunner{}
}

func (*oktetoDefaultRunner) translateConfigMapAndDeploy(ctx context.Context, data *pipeline.CfgData, c kubernetes.Interface) (*apiv1.ConfigMap, error) {
	return pipeline.TranslateConfigMapAndDeploy(ctx, data, c)
}

func (*oktetoDefaultRunner) destroyConfigMap(ctx context.Context, cfg *apiv1.ConfigMap, namespace string, c kubernetes.Interface) error {
	return configmaps.Destroy(ctx, cfg.Name, namespace, c)
}

func (*oktetoDefaultRunner) setErrorStatus(ctx context.Context, cfg *apiv1.ConfigMap, data *pipeline.CfgData, err error, c kubernetes.Interface) error {
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Destruction failed: %s", err.Error())
	return pipeline.UpdateConfigMap(ctx, cfg, data, c)
}

func (*oktetoInsideOktetoRunner) translateConfigMapAndDeploy(ctx context.Context, data *pipeline.CfgData, c kubernetes.Interface) (*apiv1.ConfigMap, error) {
	return nil, nil
}

func (*oktetoInsideOktetoRunner) destroyConfigMap(ctx context.Context, cfg *apiv1.ConfigMap, namespace string, c kubernetes.Interface) error {
	return nil
}

func (*oktetoInsideOktetoRunner) setErrorStatus(ctx context.Context, cfg *apiv1.ConfigMap, data *pipeline.CfgData, err error, c kubernetes.Interface) error {
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Destruction failed: %s", err.Error())
	return nil
}
