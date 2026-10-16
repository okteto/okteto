// Copyright 2026 The Okteto Authors
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
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd/api"
)

func dependencyConfigMap(t *testing.T, name string, envs map[string]string) *apiv1.ConfigMap {
	t.Helper()
	encoded, err := json.Marshal(envs)
	require.NoError(t, err)
	return &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipeline.TranslatePipelineName(name),
			Namespace: "test",
		},
		Data: map[string]string{
			constants.OktetoDependencyEnvsKey: base64.StdEncoding.EncodeToString(encoded),
		},
	}
}

func setTestOktetoContext() {
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test": {
				Namespace: "test",
				IsOkteto:  true,
				Cfg:       &api.Config{},
			},
		},
		CurrentContext: "test",
	}
}

func TestConfigMapHandlerGetDependencyVariableEnvVars(t *testing.T) {
	setTestOktetoContext()
	cmap := dependencyConfigMap(t, "my-db", map[string]string{"PASSWORD": "secret"})
	handler := newDefaultConfigMapHandler(test.NewFakeK8sProvider(cmap), nil)

	envs, err := handler.GetDependencyVariableEnvVars(context.Background(), "my-db", "test")

	require.NoError(t, err)
	require.Equal(t, map[string]string{"OKTETO_DEPENDENCY_MY_DB_VARIABLE_PASSWORD": "secret"}, envs)
}

func TestConfigMapHandlerGetDependencyVariableEnvVarsWithoutEnvs(t *testing.T) {
	setTestOktetoContext()
	cmap := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipeline.TranslatePipelineName("my-db"),
			Namespace: "test",
		},
	}
	handler := newDefaultConfigMapHandler(test.NewFakeK8sProvider(cmap), nil)

	envs, err := handler.GetDependencyVariableEnvVars(context.Background(), "my-db", "test")

	require.NoError(t, err)
	require.Nil(t, envs)
}

func TestConfigMapHandlerGetDependencyVariableEnvVarsSanitizesName(t *testing.T) {
	setTestOktetoContext()
	cmap := dependencyConfigMap(t, "My.DB_x", map[string]string{"PASSWORD": "secret"})
	handler := newDefaultConfigMapHandler(test.NewFakeK8sProvider(cmap), nil)

	envs, err := handler.GetDependencyVariableEnvVars(context.Background(), "My.DB_x", "test")

	require.NoError(t, err)
	require.Equal(t, map[string]string{"OKTETO_DEPENDENCY_MY_DB_X_VARIABLE_PASSWORD": "secret"}, envs)
}

func TestConfigMapHandlerGetDependencyVariableEnvVarsConfigMapNotFound(t *testing.T) {
	setTestOktetoContext()
	handler := newDefaultConfigMapHandler(test.NewFakeK8sProvider(), nil)

	_, err := handler.GetDependencyVariableEnvVars(context.Background(), "my-db", "test")

	require.True(t, k8sErrors.IsNotFound(err))
}
