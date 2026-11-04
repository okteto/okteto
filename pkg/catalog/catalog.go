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

// Package catalog checks the secret variables of the catalog item a deploy or destroy runs for, before any of its
// commands runs. The values are never read here: remote-run gets them for the commands it runs
package catalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
)

const (
	// ItemIDEnvVar is the id of the catalog item. The backend sets it on the installer job of catalog deployments
	ItemIDEnvVar = "OKTETO_CATALOG_ITEM_ID"

	// SecretVariablesEndpointEnvVar is the URL remote-run gets the values of the secret variables from. The backend
	// sets it on the installer job
	SecretVariablesEndpointEnvVar = "OKTETO_CATALOG_SECRET_VARIABLES_ENDPOINT"

	// ItemIDFlag is the remote-run flag with the id of the catalog item whose secret variables it gets
	ItemIDFlag = "catalog-item-id"

	// SecretVariablesEndpointFlag is the remote-run flag with the URL it gets the values of the secret variables from
	SecretVariablesEndpointFlag = "catalog-secret-variables-endpoint"

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
	// ErrSecretVariablesNeedRemote is returned when the catalog item has secret variables and the commands wouldn't
	// run remotely, which is the only place they get them
	ErrSecretVariablesNeedRemote = errors.New("catalog variables of type `secret` are only supported with deploy on remote. Enable remote execution and try again")

	errSecretVariablesEndpointNotSet = fmt.Errorf("the catalog item has secret variables, but %s is not set", SecretVariablesEndpointEnvVar)
)

// Item is the catalog item a deploy or destroy runs for
type Item struct {
	ID                      string
	SecretVariablesEndpoint string
}

// ItemFromEnv reads the catalog item from the env vars the backend sets on the installer job. It has to be called
// before --var, .env files, Okteto Variables or stored variables are loaded, as any of them can hold a variable
// with the same name
func ItemFromEnv() Item {
	return Item{
		ID:                      os.Getenv(ItemIDEnvVar),
		SecretVariablesEndpoint: os.Getenv(SecretVariablesEndpointEnvVar),
	}
}

// RemoteRunFlags returns the remote-run flags that make it get the values of the secret variables of the item, or
// none for an empty item
func (i Item) RemoteRunFlags() []string {
	if i.ID == "" {
		return nil
	}
	return []string{
		fmt.Sprintf("--%s %q", ItemIDFlag, i.ID),
		fmt.Sprintf("--%s %q", SecretVariablesEndpointFlag, i.SecretVariablesEndpoint),
	}
}

// SecretVariablesGetter returns the names of the secret variables of the catalog item with the id provided
type SecretVariablesGetter func(ctx context.Context, id string) ([]string, error)

// GetSecretVariables returns the names of the secret variables of a catalog item from the Okteto API of the current
// context
func GetSecretVariables(ctx context.Context, id string) ([]string, error) {
	c, err := okteto.NewOktetoClient()
	if err != nil {
		return nil, err
	}
	return c.Pipeline().GetCatalogItemSecretVariables(ctx, id)
}

// CheckSecretVariables checks the secret variables of the catalog item before the commands of the phase run. It
// returns the item remote-run has to get their values for, which is empty when the commands need none: there is no
// item, the item has no secret variables, or the item of a deploy was deleted. It fails when the secret variables
// can't be checked, or when the commands wouldn't run remotely. runsInRemote is only called when the item has
// secret variables
func CheckSecretVariables(ctx context.Context, item Item, phase Phase, getSecretVariables SecretVariablesGetter, runsInRemote func() bool) (Item, error) {
	if item.ID == "" {
		return Item{}, nil
	}

	names, err := getSecretVariables(ctx, item.ID)
	if errors.Is(err, oktetoErrors.ErrCatalogItemNotFound) {
		if phase == DeployPhase {
			// a deleted item has no secret variables, so the environments deployed from it can still be redeployed
			oktetoLog.Warning("Catalog item '%s' not found: deploying without its secret variables", item.ID)
			return Item{}, nil
		}
		return Item{}, phase.fail(fmt.Errorf("catalog item '%s' not found: can't tell whether its destroy commands need secret variables", item.ID))
	}
	if err != nil {
		return Item{}, phase.fail(fmt.Errorf("could not check the secret variables of catalog item '%s': %w", item.ID, err))
	}
	if len(names) == 0 {
		return Item{}, nil
	}

	oktetoLog.Infof("catalog item '%s' has secret variables: %s", item.ID, strings.Join(names, ", "))
	if !runsInRemote() {
		return Item{}, phase.fail(ErrSecretVariablesNeedRemote)
	}
	if item.SecretVariablesEndpoint == "" {
		return Item{}, phase.fail(errSecretVariablesEndpointNotSet)
	}
	return item, nil
}

// fail returns the error of a failed check. On destroy, it points to force destroy, which a failed check doesn't stop
func (p Phase) fail(err error) error {
	if p == DestroyPhase {
		return oktetoErrors.UserError{E: err, Hint: forceDestroyHint}
	}
	return oktetoErrors.UserError{E: err}
}
