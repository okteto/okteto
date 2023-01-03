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

package up

import (
	"context"
	"errors"
	"fmt"
	"testing"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/model/forward"
)

func Test_waitUntilExitOrInterrupt(t *testing.T) {
	up := upContext{
		Options: &UpOptions{},
	}
	up.CommandResult = make(chan error, 1)
	up.CommandResult <- nil
	ctx := context.Background()
	err := up.waitUntilExitOrInterruptOrApply(ctx)
	if err != nil {
		t.Errorf("exited with error instead of nil: %s", err)
	}

	up.CommandResult <- fmt.Errorf("custom-error")
	err = up.waitUntilExitOrInterruptOrApply(ctx)
	if err == nil {
		t.Errorf("didn't report proper error")
	}
	if _, ok := err.(oktetoErrors.CommandError); !ok {
		t.Errorf("didn't translate the error: %s", err)
	}

	up.Disconnect = make(chan error, 1)
	up.Disconnect <- oktetoErrors.ErrLostSyncthing
	err = up.waitUntilExitOrInterruptOrApply(ctx)
	if err != oktetoErrors.ErrLostSyncthing {
		t.Errorf("exited with error %s instead of %s", err, oktetoErrors.ErrLostSyncthing)
	}
}

func Test_printDisplayContext(t *testing.T) {
	var tests = []struct {
		name string
		up   *upContext
	}{
		{
			name: "basic",
			up: &upContext{
				Dev: &model.Dev{
					Name:      "dev",
					Namespace: "namespace",
				},
				Manifest: &model.Manifest{
					GlobalForward: []forward.GlobalForward{},
				},
			},
		},
		{
			name: "single-forward",
			up: &upContext{
				Dev: &model.Dev{
					Name:      "dev",
					Namespace: "namespace",
					Forward:   []forward.Forward{{Local: 1000, Remote: 1000}},
				},
				Manifest: &model.Manifest{
					GlobalForward: []forward.GlobalForward{},
				},
			},
		},
		{
			name: "multiple-forward",
			up: &upContext{
				Dev: &model.Dev{
					Name:      "dev",
					Namespace: "namespace",
					Forward:   []forward.Forward{{Local: 1000, Remote: 1000}, {Local: 2000, Remote: 2000}},
				},
				Manifest: &model.Manifest{
					GlobalForward: []forward.GlobalForward{
						{
							Local:  8080,
							Remote: 8080,
						},
						{
							Local:       8080,
							Remote:      8080,
							ServiceName: "api",
						},
					},
				},
			},
		},
		{
			name: "single-reverse",
			up: &upContext{
				Dev: &model.Dev{
					Name:      "dev",
					Namespace: "namespace",
					Reverse:   []model.Reverse{{Local: 1000, Remote: 1000}},
				},
				Manifest: &model.Manifest{
					GlobalForward: []forward.GlobalForward{},
				},
			},
		},
		{
			name: "multiple-reverse+global-forward",
			up: &upContext{
				Dev: &model.Dev{
					Name:      "dev",
					Namespace: "namespace",
					Reverse:   []model.Reverse{{Local: 1000, Remote: 1000}, {Local: 2000, Remote: 2000}},
				},
				Manifest: &model.Manifest{
					GlobalForward: []forward.GlobalForward{
						{
							Local:  8080,
							Remote: 8080,
						},
						{
							Local:       8080,
							Remote:      8080,
							ServiceName: "api",
						},
					},
				},
			},
		},
		{
			name: "global-forward",
			up: &upContext{
				Dev: &model.Dev{
					Name:      "dev",
					Namespace: "namespace",
				},
				Manifest: &model.Manifest{
					GlobalForward: []forward.GlobalForward{
						{
							Local:       8080,
							Remote:      8080,
							ServiceName: "api",
						},
					},
				},
			},
		},
		{
			name: "multiple-global-forward",
			up: &upContext{
				Dev: &model.Dev{
					Name:      "dev",
					Namespace: "namespace",
				},
				Manifest: &model.Manifest{
					GlobalForward: []forward.GlobalForward{
						{
							Local:       8080,
							Remote:      8080,
							ServiceName: "api",
						},
						{
							Local:       27017,
							Remote:      27017,
							ServiceName: "mongodb",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			printDisplayContext(tt.up)
		})
	}

}

func TestEnvVarIsAddedProperlyToDevContainerWhenIsSetFromCmd(t *testing.T) {
	var tests = []struct {
		name                    string
		dev                     *model.Dev
		upOptions               *UpOptions
		expectedNumManifestEnvs int
	}{
		{
			name:                    "Add only env vars from cmd to dev container",
			dev:                     &model.Dev{},
			upOptions:               &UpOptions{Envs: []string{"VAR1=value1", "VAR2=value2"}},
			expectedNumManifestEnvs: 2,
		},
		{
			name:                    "Add only env vars from cmd to dev container using envsubst format",
			dev:                     &model.Dev{},
			upOptions:               &UpOptions{Envs: []string{"VAR1=value1", "VAR2=${var=$USER}"}},
			expectedNumManifestEnvs: 2,
		},
		{
			name:                    "Add only env vars from cmd to dev container using non ascii characters",
			dev:                     &model.Dev{},
			upOptions:               &UpOptions{Envs: []string{"PASS=~$#@"}},
			expectedNumManifestEnvs: 1,
		},
		{
			name: "Add env vars from cmd and manifest to dev container",
			dev: &model.Dev{
				Environment: model.Environment{
					{
						Name:  "VAR_FROM_MANIFEST",
						Value: "value",
					},
				},
			},
			upOptions:               &UpOptions{Envs: []string{"VAR1=value1", "VAR2=value2"}},
			expectedNumManifestEnvs: 3,
		},
		{
			name: "Overwrite env vars when is required",
			dev: &model.Dev{
				Environment: model.Environment{
					{
						Name:  "VAR_TO_OVERWRITE",
						Value: "oldValue",
					},
				},
			},
			upOptions:               &UpOptions{Envs: []string{"VAR_TO_OVERWRITE=newValue", "VAR2=value2"}},
			expectedNumManifestEnvs: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			overridedEnvVars, err := getOverridedEnvVarsFromCmd(tt.dev.Environment, tt.upOptions.Envs)
			if err != nil {
				t.Fatalf("unexpected error in  setEnvVarsFromCmd: %s", err)
			}

			if tt.expectedNumManifestEnvs != len(*overridedEnvVars) {
				t.Fatalf("error in setEnvVarsFromCmd; expected num variables in container %d but got %d", tt.expectedNumManifestEnvs, len(tt.dev.Environment))
			}
		})
	}
}

func TestEnvVarIsNotAddedWhenHasBuiltInOktetoEnvVarsFormat(t *testing.T) {
	var tests = []struct {
		name                    string
		dev                     *model.Dev
		upOptions               *UpOptions
		expectedNumManifestEnvs int
	}{
		{
			name:                    "Unable to set built-in okteto environment variable OKTETO_NAMESPACE",
			dev:                     &model.Dev{},
			upOptions:               &UpOptions{Envs: []string{"OKTETO_NAMESPACE=value"}},
			expectedNumManifestEnvs: 2,
		},
		{
			name:                    "Unable to set built-in okteto environment variable OKTETO_GIT_BRANCH",
			dev:                     &model.Dev{},
			upOptions:               &UpOptions{Envs: []string{"OKTETO_GIT_BRANCH=value"}},
			expectedNumManifestEnvs: 2,
		},
		{
			name:                    "Unable to set built-in okteto environment variable OKTETO_GIT_COMMIT",
			dev:                     &model.Dev{},
			upOptions:               &UpOptions{Envs: []string{"OKTETO_GIT_COMMIT=value"}},
			expectedNumManifestEnvs: 2,
		},
		{
			name:                    "Unable to set built-in okteto environment variable OKTETO_REGISTRY_URL",
			dev:                     &model.Dev{},
			upOptions:               &UpOptions{Envs: []string{"OKTETO_REGISTRY_URL=value"}},
			expectedNumManifestEnvs: 2,
		},
		{
			name:                    "Unable to set built-in okteto environment variable BUILDKIT_HOST",
			dev:                     &model.Dev{},
			upOptions:               &UpOptions{Envs: []string{"BUILDKIT_HOST=value"}},
			expectedNumManifestEnvs: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := getOverridedEnvVarsFromCmd(tt.dev.Environment, tt.upOptions.Envs)
			if !errors.Is(err, oktetoErrors.ErrBuiltInOktetoEnvVarSetFromCMD) {
				t.Fatalf("expected error in setEnvVarsFromCmd: %s due to try to set a built-in okteto environment variable", err)
			}
		})
	}
}
