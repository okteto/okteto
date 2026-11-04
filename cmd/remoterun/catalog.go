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

package remoterun

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/okteto/okteto/pkg/catalog"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/cobra"
)

const (
	// catalogSecretVariablesStage is the stage of the errors getting the secret
	// variables. The deploy or destroy running remote-run drops the lines
	// logged without a stage
	catalogSecretVariablesStage = "Load catalog secret variables"

	// reservedVariablePrefix is the prefix of the variables set by Okteto, such
	// as OKTETO_TOKEN, that a catalog item must not override
	reservedVariablePrefix = "OKTETO_"

	// catalogSecretLabel is the label a Secret needs before catalog items can
	// read it
	catalogSecretLabel = "dev.okteto.com/catalog-secret=true"

	deployRetryHint  = "then deploy again."
	destroyRetryHint = "then destroy again, or use force destroy to remove the environment even if the destroy commands can't run."
)

// envVarNameRegex matches the names that can be set as env vars
var envVarNameRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// catalogSecretVariablesGetter gets the values of the secret variables of a catalog item
type catalogSecretVariablesGetter interface {
	GetCatalogSecretVariables(ctx context.Context, endpoint, catalogItemID string) ([]types.CatalogSecretVariable, error)
}

// catalogSecretVariables gets the secret variables of the catalog item being
// deployed or destroyed, to inject them into the env of the commands
type catalogSecretVariables struct {
	client   catalogSecretVariablesGetter
	itemID   string
	endpoint string
	// retryHint ends the hints, telling how to run the operation again
	retryHint string
}

// invalidCatalogSecretVariableNamesError is returned when the catalog item has
// secret variables that can't be injected because of their names
type invalidCatalogSecretVariableNamesError struct {
	// lines has a line per invalid variable explaining why
	lines []string
}

func (invalidCatalogSecretVariableNamesError) Error() string {
	return "the secret variables of the catalog item have invalid names"
}

// addCatalogSecretVariablesFlags adds the flags to get the secret variables of
// the catalog item. They are only read from these flags, so a variable can't
// change them
func addCatalogSecretVariablesFlags(cmd *cobra.Command, itemID, endpoint *string) {
	cmd.Flags().StringVar(itemID, catalog.ItemIDFlag, "", "id of the catalog item whose secret variables are injected into the commands")
	cmd.Flags().StringVar(endpoint, catalog.SecretVariablesEndpointFlag, "", "endpoint to get the secret variables of the catalog item")
}

// newCatalogSecretVariables returns the getter of the secret variables of the
// catalog item. Without a catalog item id, nothing is fetched
func newCatalogSecretVariables(itemID, endpoint, retryHint string) (catalogSecretVariables, error) {
	c := catalogSecretVariables{itemID: itemID, endpoint: endpoint, retryHint: retryHint}
	if itemID == "" {
		return c, nil
	}
	if endpoint == "" {
		return c, fmt.Errorf("--%s is required with --%s", catalog.SecretVariablesEndpointFlag, catalog.ItemIDFlag)
	}
	oktetoClient, err := okteto.NewOktetoClient()
	if err != nil {
		return c, err
	}
	c.client = oktetoClient.User()
	return c, nil
}

// get returns the secret variables of the catalog item as "key=value" vars.
// Any error is logged with the variables that caused it and a hint, under its
// own stage, so the deploy or destroy running remote-run shows them
func (c catalogSecretVariables) get(ctx context.Context) ([]string, error) {
	if c.itemID == "" {
		return nil, nil
	}

	variables, err := c.client.GetCatalogSecretVariables(ctx, c.endpoint, c.itemID)
	if err == nil {
		err = validateCatalogSecretVariableNames(variables)
	}
	if err != nil {
		c.logError(err)
		return nil, err
	}

	vars := make([]string, 0, len(variables))
	for _, v := range variables {
		vars = append(vars, fmt.Sprintf("%s=%s", v.Name, v.Value))
	}
	return vars, nil
}

func (c catalogSecretVariables) logError(err error) {
	oktetoLog.SetStage(catalogSecretVariablesStage)
	var unresolved okteto.UnresolvedCatalogSecretVariablesError
	var invalidNames invalidCatalogSecretVariableNamesError
	switch {
	case errors.As(err, &unresolved):
		for _, v := range unresolved.Variables {
			oktetoLog.Println(unresolvedCatalogSecretVariableMessage(v))
		}
		oktetoLog.Println(fmt.Sprintf("Ask an Okteto admin to make sure the Secret exists in the Okteto namespace with the label %s, or to fix the reference in the catalog item, %s", catalogSecretLabel, c.retryHint))
	case errors.As(err, &invalidNames):
		for _, line := range invalidNames.lines {
			oktetoLog.Println(line)
		}
		oktetoLog.Println(fmt.Sprintf("Ask an Okteto admin to rename the variable in the catalog item, %s", c.retryHint))
	}
	oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, "%s", err.Error())
}

func unresolvedCatalogSecretVariableMessage(v okteto.UnresolvedCatalogSecretVariable) string {
	cause := v.Reason
	switch v.Reason {
	case okteto.CatalogSecretNotFoundReason:
		cause = fmt.Sprintf("Secret %q not found", v.SecretKeyRef.Name)
	case okteto.CatalogSecretNotLabeledReason:
		cause = fmt.Sprintf("Secret %q isn't labelled %s", v.SecretKeyRef.Name, catalogSecretLabel)
	case okteto.CatalogSecretKeyNotFoundReason:
		cause = fmt.Sprintf("key %q not found in Secret %q", v.SecretKeyRef.Key, v.SecretKeyRef.Name)
	}
	return fmt.Sprintf("Catalog variable %q could not be resolved: %s", v.Variable, cause)
}

// validateCatalogSecretVariableNames fails if a name can't be set as an env
// var or would override a variable set by Okteto. The names come from the
// endpoint, so they are checked again before any of them reaches the env of
// the commands
func validateCatalogSecretVariableNames(variables []types.CatalogSecretVariable) error {
	var lines []string
	for _, v := range variables {
		switch {
		case !envVarNameRegex.MatchString(v.Name):
			lines = append(lines, fmt.Sprintf("Catalog variable %q can't be injected: it isn't a valid environment variable name", v.Name))
		case strings.HasPrefix(v.Name, reservedVariablePrefix):
			lines = append(lines, fmt.Sprintf("Catalog variable %q can't be injected: names starting with %s are reserved", v.Name, reservedVariablePrefix))
		}
	}
	if len(lines) > 0 {
		return invalidCatalogSecretVariableNamesError{lines: lines}
	}
	return nil
}
