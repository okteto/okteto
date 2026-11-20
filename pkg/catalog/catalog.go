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

// Package catalog checks the catalog secret variables of the development environment a deploy or destroy runs for,
// before any of its commands runs. The values are never read here: remote-run gets them for the commands it runs
package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/remote"
)

const (
	// SecretVariablesEnvVar holds the references of the catalog secret variables of the development environment: a
	// JSON array with their names and the keys of the Secrets their values come from, never the values. The backend
	// sets it on the installer job
	SecretVariablesEnvVar = "OKTETO_CATALOG_SECRET_VARIABLES"

	// SecretVariablesEndpointEnvVar is the URL remote-run gets the values of the secret variables from. The backend
	// sets it on the installer job
	SecretVariablesEndpointEnvVar = "OKTETO_CATALOG_SECRET_VARIABLES_ENDPOINT"

	// SecretVariablesTokenFileEnvVar is the path of the file with the token remote-run gets the values of the secret
	// variables with: a short-lived Kubernetes token bound to the installer pod, which the kubelet refreshes. The
	// backend sets it on the installer job. Older installations don't, and their endpoint takes the user's token
	SecretVariablesTokenFileEnvVar = "OKTETO_CATALOG_SECRET_VARIABLES_TOKEN_FILE"

	// SecretVariablesTokenSecretID is the id of the build secret that forwards the token file to remote-run. Only the
	// id reaches the Dockerfile
	SecretVariablesTokenSecretID = "catalog-secret-variables-token"

	// SecretVariablesTokenPath is where remote-run reads the token from: BuildKit's default target for the secret
	SecretVariablesTokenPath = "/run/secrets/" + SecretVariablesTokenSecretID

	// ActionFlag is the remote-run flag with the action whose catalog secret variables it gets
	ActionFlag = "catalog-action"

	// SecretVariablesEndpointFlag is the remote-run flag with the URL it gets the values of the secret variables from
	SecretVariablesEndpointFlag = "catalog-secret-variables-endpoint"

	// SecretVariablesStage is the log stage of the errors and warnings about the catalog secret variables. The json
	// output drops the lines logged without a stage
	SecretVariablesStage = "Load catalog secret variables"

	// The installer doesn't clone the repository on force destroy, so no destroy command runs
	forceDestroyHint = "Use force destroy to remove the development environment anyway: it deletes its resources without running the destroy commands"
)

// Phase is the operation whose commands need the secret variables
type Phase string

const (
	// DeployPhase runs the deploy commands
	DeployPhase Phase = "deploy"
	// DestroyPhase runs the destroy commands
	DestroyPhase Phase = "destroy"
)

var (
	// ErrSecretVariablesNeedRemote is returned when the development environment has catalog secret variables and the
	// commands wouldn't run remotely, which is the only place they get them
	ErrSecretVariablesNeedRemote = errors.New("catalog variables of type `secret` are only supported with deploy on remote. Enable remote execution and try again")

	errSecretVariablesEndpointNotSet = fmt.Errorf("the development environment has catalog secret variables, but %s is not set", SecretVariablesEndpointEnvVar)
	errActionNotSet                  = fmt.Errorf("the development environment has catalog secret variables, but %s is not set", model.OktetoActionNameEnvVar)
)

// SecretVariables are the catalog secret variables of the development environment a deploy or destroy runs for
type SecretVariables struct {
	// References is the JSON array of the references of the secret variables
	References string
	// Action is the action the installer job runs. remote-run gets the values of the secret variables by it
	Action string
	// Endpoint is the URL remote-run gets the values of the secret variables from
	Endpoint string
	// TokenFile is the path of the file with the token remote-run gets the values of the secret variables with. It's
	// empty on older installations
	TokenFile string
}

// secretVariableReference is the reference of a catalog secret variable. Only its name is read here
type secretVariableReference struct {
	Name string `json:"name"`
}

// SecretVariablesFromEnv reads the catalog secret variables from the env vars the backend sets on the installer job.
// It has to be called before --var, .env files, Okteto Variables or stored variables are loaded, as any of them can
// hold a variable with the same name
func SecretVariablesFromEnv() SecretVariables {
	return SecretVariables{
		References: os.Getenv(SecretVariablesEnvVar),
		Action:     os.Getenv(model.OktetoActionNameEnvVar),
		Endpoint:   os.Getenv(SecretVariablesEndpointEnvVar),
		TokenFile:  os.Getenv(SecretVariablesTokenFileEnvVar),
	}
}

// RemoteRunFlags returns the remote-run flags that make it get the values of the secret variables, or none when
// there is no action to get them for
func (s SecretVariables) RemoteRunFlags() []string {
	if s.Action == "" {
		return nil
	}
	return []string{
		remote.CommandFlag(ActionFlag, s.Action),
		remote.CommandFlag(SecretVariablesEndpointFlag, s.Endpoint),
	}
}

// RemoteRunSecrets returns the build secrets the remote-run RUN mounts to get the values of the secret variables: the
// token file, which BuildKit reads when the RUN mounts it. It returns none when there is no action to get them for,
// or on older installations without the token file, where remote-run gets them with the user's token
func (s SecretVariables) RemoteRunSecrets() []remote.FileSecret {
	if s.Action == "" || s.TokenFile == "" {
		return nil
	}
	return []remote.FileSecret{{ID: SecretVariablesTokenSecretID, Path: s.TokenFile}}
}

// names returns the names of the secret variables in the references
func (s SecretVariables) names() ([]string, error) {
	var references []secretVariableReference
	if err := json.Unmarshal([]byte(s.References), &references); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(references))
	for _, r := range references {
		names = append(names, r.Name)
	}
	return names, nil
}

// CheckSecretVariables checks the catalog secret variables before the commands of the phase run. It returns the ones
// remote-run has to get the values of, which are empty when the development environment has none. It fails when
// their references can't be read, or when the commands wouldn't run remotely. runsInRemote is only called when the
// development environment has secret variables
func CheckSecretVariables(s SecretVariables, phase Phase, runsInRemote func() bool) (SecretVariables, error) {
	if s.References == "" {
		return SecretVariables{}, nil
	}

	names, err := s.names()
	if err != nil {
		return SecretVariables{}, phase.fail(fmt.Errorf("could not read the catalog secret variables from %s: %w", SecretVariablesEnvVar, err))
	}
	if len(names) == 0 {
		return SecretVariables{}, nil
	}

	oktetoLog.Infof("the development environment has catalog secret variables: %s", strings.Join(names, ", "))
	if !runsInRemote() {
		return SecretVariables{}, phase.fail(ErrSecretVariablesNeedRemote)
	}
	if s.Endpoint == "" {
		return SecretVariables{}, phase.fail(errSecretVariablesEndpointNotSet)
	}
	if s.Action == "" {
		return SecretVariables{}, phase.fail(errActionNotSet)
	}
	return s, nil
}

// fail returns the error of a failed check. On destroy, it points to force destroy, which a failed check doesn't stop
func (p Phase) fail(err error) error {
	if p == DestroyPhase {
		return oktetoErrors.UserError{E: err, Hint: forceDestroyHint}
	}
	return oktetoErrors.UserError{E: err}
}
