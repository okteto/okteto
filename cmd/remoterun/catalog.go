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
	catalogSecretVariablesStage = catalog.SecretVariablesStage

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

// catalogSecretVariablesGetter gets the catalog secret variables of the
// development environment an action runs for
type catalogSecretVariablesGetter interface {
	GetCatalogSecretVariables(ctx context.Context, endpoint, action string) (types.CatalogSecretVariables, error)
}

// catalogSecretVariables gets the catalog secret variables of the development
// environment being deployed or destroyed, to inject them into the env of the
// commands
type catalogSecretVariables struct {
	client   catalogSecretVariablesGetter
	action   string
	endpoint string
	// phase is the operation whose commands get the variables
	phase catalog.Phase
	// forceDestroy runs the destroy commands without the variables when they
	// can't be got, as a forced destroy runs them despite their errors
	forceDestroy bool
}

// unresolvedCatalogSecretVariablesError is returned when some secret variables
// can't be resolved. It lists all of them
type unresolvedCatalogSecretVariablesError struct {
	variables []types.UnresolvedCatalogSecretVariable
}

func (unresolvedCatalogSecretVariablesError) Error() string {
	return "the catalog secret variables could not be resolved"
}

// invalidCatalogSecretVariableNamesError is returned when the development
// environment has secret variables that can't be injected because of their
// names
type invalidCatalogSecretVariableNamesError struct {
	// lines has a line per invalid variable explaining why
	lines []string
}

func (invalidCatalogSecretVariableNamesError) Error() string {
	return "the catalog secret variables have invalid names"
}

// addCatalogSecretVariablesFlags adds the flags to get the catalog secret
// variables. They are only read from these flags, so a variable can't change
// them
func addCatalogSecretVariablesFlags(cmd *cobra.Command, action, endpoint *string) {
	cmd.Flags().StringVar(action, catalog.ActionFlag, "", "action whose catalog secret variables are injected into the commands")
	cmd.Flags().StringVar(endpoint, catalog.SecretVariablesEndpointFlag, "", "endpoint to get the catalog secret variables from")
}

// newCatalogSecretVariables returns the getter of the catalog secret variables
// of the action, for the commands of the phase. Without an action, nothing is
// fetched
func newCatalogSecretVariables(action, endpoint string, phase catalog.Phase) (catalogSecretVariables, error) {
	c := catalogSecretVariables{action: action, endpoint: endpoint, phase: phase}
	if action == "" {
		return c, nil
	}
	if endpoint == "" {
		return c, fmt.Errorf("--%s is required with --%s", catalog.SecretVariablesEndpointFlag, catalog.ActionFlag)
	}
	oktetoClient, err := okteto.NewOktetoClient()
	if err != nil {
		return c, err
	}
	c.client = oktetoClient.User()
	return c, nil
}

// get returns the catalog secret variables as "key=value" vars. Any error, and
// the warnings about the variables skipped, are logged with the variables that
// caused them under their own stage, so the deploy or destroy running
// remote-run shows them
func (c catalogSecretVariables) get(ctx context.Context) ([]string, error) {
	if c.action == "" {
		return nil, nil
	}

	variables, err := c.client.GetCatalogSecretVariables(ctx, c.endpoint, c.action)
	if err == nil && len(variables.Unresolved) > 0 && !c.skipsUnresolved() {
		err = unresolvedCatalogSecretVariablesError{variables: variables.Unresolved}
	}
	if err == nil {
		err = validateCatalogSecretVariableNames(variables.Variables)
	}
	if err != nil && c.forceDestroy {
		c.logSkipped(err)
		return nil, nil
	}
	if err != nil {
		c.logError(err)
		return nil, err
	}

	if len(variables.Unresolved) > 0 {
		stage := oktetoLog.GetStage()
		oktetoLog.SetStage(catalogSecretVariablesStage)
		for _, v := range variables.Unresolved {
			oktetoLog.Warning("%s. The commands run without it", unresolvedCatalogSecretVariableMessage(v))
		}
		oktetoLog.SetStage(stage)
	}

	vars := make([]string, 0, len(variables.Variables))
	for _, v := range variables.Variables {
		vars = append(vars, fmt.Sprintf("%s=%s", v.Name, v.Value))
	}
	return vars, nil
}

// skipsUnresolved is true when the commands run without the variables that
// can't be resolved, with a warning per variable, instead of failing. Only
// destroy does: deploy always fails
func (c catalogSecretVariables) skipsUnresolved() bool {
	return c.phase == catalog.DestroyPhase
}

// retryHint ends the hints, telling how to run the operation again
func (c catalogSecretVariables) retryHint() string {
	if c.phase == catalog.DestroyPhase {
		return destroyRetryHint
	}
	return deployRetryHint
}

func (c catalogSecretVariables) logError(err error) {
	oktetoLog.SetStage(catalogSecretVariablesStage)
	var unresolved unresolvedCatalogSecretVariablesError
	var invalidNames invalidCatalogSecretVariableNamesError
	switch {
	case errors.As(err, &unresolved):
		for _, v := range unresolved.variables {
			oktetoLog.Println(unresolvedCatalogSecretVariableMessage(v))
		}
		oktetoLog.Println(fmt.Sprintf("Ask an Okteto admin to make sure the Secret exists in the Okteto namespace with the label %s, or to fix the reference in the catalog item, %s", catalogSecretLabel, c.retryHint()))
	case errors.As(err, &invalidNames):
		for _, line := range invalidNames.lines {
			oktetoLog.Println(line)
		}
		oktetoLog.Println(fmt.Sprintf("Ask an Okteto admin to rename the variable in the catalog item, %s", c.retryHint()))
	}
	oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, "%s", err.Error())
}

// logSkipped logs, as warnings, why a forced destroy runs its commands without
// the catalog secret variables
func (catalogSecretVariables) logSkipped(err error) {
	stage := oktetoLog.GetStage()
	oktetoLog.SetStage(catalogSecretVariablesStage)
	var invalidNames invalidCatalogSecretVariableNamesError
	if errors.As(err, &invalidNames) {
		for _, line := range invalidNames.lines {
			oktetoLog.Warning("%s", line)
		}
	}
	oktetoLog.Warning("Running the destroy commands without the catalog secret variables: %s", err)
	oktetoLog.SetStage(stage)
}

func unresolvedCatalogSecretVariableMessage(v types.UnresolvedCatalogSecretVariable) string {
	cause := v.Reason
	switch v.Reason {
	case types.CatalogSecretNotFoundReason:
		cause = fmt.Sprintf("Secret %q not found", v.SecretKeyRef.Name)
	case types.CatalogSecretNotLabeledReason:
		cause = fmt.Sprintf("Secret %q isn't labelled %s", v.SecretKeyRef.Name, catalogSecretLabel)
	case types.CatalogSecretKeyNotFoundReason:
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
