package remoterun

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/deployable"
	"github.com/okteto/okteto/pkg/externalresource"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fakeRunner struct {
	mock.Mock
}

func (f *fakeRunner) RunDeploy(ctx context.Context, params deployable.DeployParameters) error {
	args := f.Called(ctx, params)
	return args.Error(0)
}

func TestGetDeployableEmpty(t *testing.T) {
	dep, err := getDeployable()

	expected := deployable.Entity{
		Commands: []model.DeployCommand{},
	}

	require.NoError(t, err)
	require.Equal(t, expected, dep)
}

func TestGetDeployableBadBase64(t *testing.T) {
	t.Setenv(constants.OktetoDeployableEnvVar, "bad-base64  ** %")

	_, err := getDeployable()

	require.Error(t, err)
}

func TestGetDeployableInvalidDeployable(t *testing.T) {
	t.Setenv(constants.OktetoDeployableEnvVar, "aW52YWxpZCBkZXBsb3lhYmxlCg==")

	_, err := getDeployable()

	require.Error(t, err)
}

func TestGetDeployable(t *testing.T) {
	t.Setenv(constants.OktetoDeployableEnvVar, "Y29tbWFuZHM6CiAgLSBuYW1lOiBDb21tYW5kIDEKICAgIGNvbW1hbmQ6IGVjaG8gMQogIC0gbmFtZTogQ29tbWFuZCAyCiAgICBjb21tYW5kOiBlY2hvIDIKZXh0ZXJuYWw6CiAgZmFrZToKICAgIGljb246IGljb24KICAgIGVuZHBvaW50czoKICAgIC0gbmFtZTogbmFtZQogICAgICB1cmw6IHVybApkaXZlcnQ6CiAgZHJpdmVyOiAidGVzdCBkcml2ZXIiCiAgbmFtZXNwYWNlOiBucwo=")

	expected := deployable.Entity{
		Commands: []model.DeployCommand{
			{
				Name:    "Command 1",
				Command: "echo 1",
			},
			{
				Name:    "Command 2",
				Command: "echo 2",
			},
		},
		External: externalresource.Section{
			"fake": {
				Icon: "icon",
				Endpoints: []*externalresource.ExternalEndpoint{
					{
						Name: "name",
						Url:  "url",
					},
				},
			},
		},
		Divert: &model.DivertDeploy{
			Driver:    "test driver",
			Namespace: "ns",
		},
	}

	result, err := getDeployable()

	require.NoError(t, err)
	require.Equal(t, expected, result)
}

func TestRun(t *testing.T) {
	params := deployable.DeployParameters{
		Name:      "test",
		Namespace: "ns",
		Deployable: deployable.Entity{
			Commands: []model.DeployCommand{
				{
					Name:    "Command 1",
					Command: "echo 1",
				},
				{
					Name:    "Command 2",
					Command: "echo 2",
				},
			},
			External: externalresource.Section{
				"fake": {
					Icon: "icon",
					Endpoints: []*externalresource.ExternalEndpoint{
						{
							Name: "name",
							Url:  "url",
						},
					},
				},
			},
			Divert: &model.DivertDeploy{
				Driver:    "test driver",
				Namespace: "ns",
			},
		},
	}
	runner := &fakeRunner{}

	c := &Command{
		runner: runner,
	}

	runner.On("RunDeploy", mock.Anything, params).Return(nil)

	err := c.Run(context.Background(), params)

	assert.NoError(t, err)
	runner.AssertExpectations(t)
}
