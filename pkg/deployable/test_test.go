// Copyright 2024 The Okteto Authors
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

package deployable

import (
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestTestRunner(t *testing.T) {
	executor := &fakeExecutor{}
	runner := TestRunner{
		Executor: executor,
		Fs:       afero.NewMemMapFs(),
	}

	cmd1 := model.DeployCommand{
		Name:    "run command",
		Command: "echo this_is_my_command",
	}

	expectedVars := []string{
		"DEPLOY_ENV_1=deploy1",
		"DEPLOY_ENV_2=deploy2",
	}
	executor.On("Execute", cmd1, expectedVars).Return(nil).Once()

	err := runner.RunTest(TestParameters{
		Name:      "test",
		Namespace: "ns",
		Deployable: Entity{
			Commands: []model.DeployCommand{cmd1},
		},
		Variables: expectedVars,
	})

	require.NoError(t, err)
	require.True(t, executor.AssertExpectations(t))
}
