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

package remoterun

import (
	"testing"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/deployable"
	"github.com/okteto/okteto/pkg/externalresource"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/require"
)

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
