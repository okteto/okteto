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
		GetDevEnvEnviron: func(devEnvName, namespace string) (map[string]string, error) {
			return map[string]string{
				"DEPLOY_ENV_1": "deploy1",
				"DEPLOY_ENV_2": "deploy2",
			}, nil
		},
		SetDevEnvEnviron: func(devEnvName, namespace string, vars []string) error {
			return nil
		},
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
		Name:       "test",
		Namespace:  "ns",
		DevEnvName: "deploy",
		Deployable: Entity{
			Commands: []model.DeployCommand{cmd1},
		},
	})

	require.NoError(t, err)
	require.True(t, executor.AssertExpectations(t))
}
