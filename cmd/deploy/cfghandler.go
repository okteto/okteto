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
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/format"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	apiv1 "k8s.io/api/core/v1"
)

// ConfigMapHandler defines the different functions to run okteto inside an okteto deploy
// or an okteto destroy directly
type ConfigMapHandler interface {
	TranslateConfigMapAndDeploy(context.Context, *pipeline.CfgData) (*apiv1.ConfigMap, error)
	UpdateConfigMap(context.Context, *apiv1.ConfigMap, *pipeline.CfgData, error) error
	UpdateEnvsFromCommands(context.Context, string, string, []string) error
	GetDependencyBuildEnvVars(context.Context, string, string) (map[string]string, error)
	SetBuildEnvVars(context.Context, string, string, map[string]string) error
	GetConfigmapVariablesEncoded(ctx context.Context, name, namespace string) (string, error)
	AddPhaseDuration(context.Context, string, string, string, time.Duration) error
}

// oktetoDefaultConfigMapHandler is the runner used when the okteto is executed
// directly
type defaultConfigMapHandler struct {
	k8sClientProvider okteto.K8sClientProviderWithLogger
	k8slogger         *io.K8sLogger
}

func newDefaultConfigMapHandler(provider okteto.K8sClientProviderWithLogger, k8slogger *io.K8sLogger) *defaultConfigMapHandler {
	return &defaultConfigMapHandler{
		k8sClientProvider: provider,
		k8slogger:         k8slogger,
	}
}

func NewConfigmapHandler(provider okteto.K8sClientProviderWithLogger, k8slogger *io.K8sLogger) ConfigMapHandler {
	return newDefaultConfigMapHandler(provider, k8slogger)
}

func (ch *defaultConfigMapHandler) TranslateConfigMapAndDeploy(ctx context.Context, data *pipeline.CfgData) (*apiv1.ConfigMap, error) {
	c, _, err := ch.k8sClientProvider.ProvideWithLogger(okteto.GetContext().Cfg, ch.k8slogger)
	if err != nil {
		return nil, err
	}
	return pipeline.TranslateConfigMapAndDeploy(ctx, data, c)
}

func (ch *defaultConfigMapHandler) GetConfigmapVariablesEncoded(ctx context.Context, name, namespace string) (string, error) {
	c, _, err := ch.k8sClientProvider.ProvideWithLogger(okteto.GetContext().Cfg, ch.k8slogger)
	if err != nil {
		return "", err
	}
	return pipeline.GetConfigmapVariablesEncoded(ctx, name, namespace, c)
}

func (ch *defaultConfigMapHandler) UpdateConfigMap(ctx context.Context, cfg *apiv1.ConfigMap, data *pipeline.CfgData, errMain error) error {
	c, _, err := ch.k8sClientProvider.ProvideWithLogger(okteto.GetContext().Cfg, ch.k8slogger)
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

// UpdateEnvsFromCommands update config map by adding envs generated in OKTETO_ENV as data fields
func (ch *defaultConfigMapHandler) UpdateEnvsFromCommands(ctx context.Context, name, namespace string, envs []string) error {
	c, _, err := ch.k8sClientProvider.ProvideWithLogger(okteto.GetContext().Cfg, ch.k8slogger)
	if err != nil {
		return err
	}

	if err := pipeline.UpdateEnvs(ctx, name, namespace, envs, c); err != nil {
		return err
	}
	return nil
}

func (ch *defaultConfigMapHandler) AddPhaseDuration(ctx context.Context, name, namespace, phase string, duration time.Duration) error {
	c, _, err := ch.k8sClientProvider.ProvideWithLogger(okteto.GetContext().Cfg, ch.k8slogger)
	if err != nil {
		return err
	}
	return pipeline.AddPhaseDuration(ctx, name, namespace, phase, duration, c)
}

func (ch *defaultConfigMapHandler) SetBuildEnvVars(ctx context.Context, name, ns string, envVars map[string]string) error {
	c, _, err := ch.k8sClientProvider.ProvideWithLogger(okteto.GetContext().Cfg, ch.k8slogger)
	if err != nil {
		return err
	}

	re := regexp.MustCompile(`^OKTETO_BUILD_(.+)_(REGISTRY|REPOSITORY|IMAGE|TAG|SHA)$`)
	buildEnvVars := make(map[string]map[string]string)
	for k, v := range envVars {
		// Ignore env vars that are not related to build
		if !re.MatchString(k) {
			continue
		}

		// Extract the build name and variable name from the env var
		// e.g. OKTETO_BUILD_myapp_REGISTRY -> myapp, REGISTRY
		// e.g. OKTETO_BUILD_myapp_REPOSITORY -> myapp, REPOSITORY
		// e.g. OKTETO_BUILD_myapp_TAG -> myapp, TAG
		// e.g. OKTETO_BUILD_myapp_SHA -> myapp, SHA
		// e.g. OKTETO_BUILD_myapp_IMAGE -> myapp, IMAGE
		m := re.FindStringSubmatch(k)
		if m == nil {
			continue
		}
		name, varName := m[1], m[2]
		if _, ok := buildEnvVars[name]; !ok {
			buildEnvVars[name] = make(map[string]string)
		}

		buildEnvVars[name][varName] = v
	}

	return pipeline.SetBuildEnvVars(ctx, name, ns, buildEnvVars, c)
}

func (ch *defaultConfigMapHandler) GetDependencyBuildEnvVars(ctx context.Context, name, ns string) (map[string]string, error) {
	c, _, err := ch.k8sClientProvider.ProvideWithLogger(okteto.GetContext().Cfg, ch.k8slogger)
	if err != nil {
		return nil, err
	}
	envVars, err := pipeline.GetConfigmapBuildEnvVars(ctx, name, ns, c)
	if err != nil {
		return nil, err
	}
	if len(envVars) == 0 {
		return nil, nil
	}
	dependencyName := strings.ToUpper(strings.ReplaceAll(format.ResourceK8sMetaString(name), "-", "_"))
	envs := make(map[string]string)
	for build, buildEnvVarValue := range envVars {
		for buildEnvVar, value := range buildEnvVarValue {
			dependencyName := fmt.Sprintf("OKTETO_DEPENDENCY_%s_BUILD_%s_%s", dependencyName, build, buildEnvVar)
			envs[dependencyName] = value
		}
	}
	return envs, nil
}
