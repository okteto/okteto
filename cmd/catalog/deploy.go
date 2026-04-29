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

package catalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/agext/levenshtein"
	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/okteto/okteto/pkg/validator"
	"github.com/spf13/cobra"
)

const (
	deployTimeoutDefault = 5 * time.Minute
	varKV                = 2
	// suggestionMaxDistance caps how tolerant the "did you mean?" suggestion is.
	// Names within this Levenshtein distance are considered similar enough to suggest.
	suggestionMaxDistance = 3
)

// errCatalogItemNotFound is returned when no catalog item matches the requested name.
var errCatalogItemNotFound = errors.New("catalog item not found")

// errNoCatalogItems is returned when deploy is invoked interactively but the
// catalog is empty, so there is nothing to pick from.
var errNoCatalogItems = oktetoErrors.UserError{
	E:    errors.New("no catalog items are available"),
	Hint: "Ask your administrator to add a catalog item, or run 'okteto catalog add' to open the Okteto UI.",
}

type deployFlags struct {
	name       string
	namespace  string
	k8sContext string
	branch     string
	file       string
	variables  []string
	timeout    time.Duration
	wait       bool
}

// Deploy returns the `okteto catalog deploy <name>` cobra command.
func Deploy(ctx context.Context) *cobra.Command {
	flags := &deployFlags{}
	cmd := &cobra.Command{
		Use:   "deploy <catalog-item>",
		Short: "Deploy a Development Environment from the Okteto Catalog",
		Long: `Deploy a Development Environment from an Okteto Catalog item.

The catalog item's repository, branch, manifest path, and default variables are
used to create the Development Environment in your current Okteto Namespace.
Use --var KEY=VALUE to override any default variable.`,
		Args: utils.MaximumNArgsAccepted(1, ""),
		Example: `Run without arguments to pick an item from an interactive list:
  okteto catalog deploy

Deploy a catalog item by name into the current namespace:
  okteto catalog deploy demo-app

Override a variable and deploy into a specific namespace:
  okteto catalog deploy demo-app -n my-ns --var API_TOKEN=xyz`,
		RunE: func(cmd *cobra.Command, args []string) error {
			catalogItemName := ""
			if len(args) > 0 {
				catalogItemName = args[0]
			}

			if err := validator.CheckReservedVariablesNameOption(flags.variables); err != nil {
				return err
			}

			ctxOptions := &contextCMD.Options{
				Namespace: flags.namespace,
				Context:   flags.k8sContext,
				Show:      true,
			}
			if err := contextCMD.NewContextCommand().Run(ctx, ctxOptions); err != nil {
				return err
			}
			if !okteto.IsOkteto() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}

			c, err := NewCommand()
			if err != nil {
				return err
			}
			return c.ExecuteDeploy(ctx, catalogItemName, flags)
		},
	}

	cmd.Flags().StringVar(&flags.name, "name", "", "name of the deployed Development Environment (defaults to the catalog item name)")
	cmd.Flags().StringVarP(&flags.namespace, "namespace", "n", "", "namespace to deploy into (defaults to your current Okteto Namespace)")
	cmd.Flags().StringVarP(&flags.k8sContext, "context", "c", "", "overwrite the current Okteto Context")
	cmd.Flags().StringVarP(&flags.branch, "branch", "b", "", "override the branch configured on the catalog item")
	cmd.Flags().StringVarP(&flags.file, "file", "f", "", "override the manifest path configured on the catalog item")
	cmd.Flags().StringArrayVarP(&flags.variables, "var", "v", []string{}, "set a variable to override the catalog item default (can be set more than once, KEY=VALUE)")
	cmd.Flags().DurationVarP(&flags.timeout, "timeout", "t", deployTimeoutDefault, "how long to wait for the deployment to complete (e.g. 1s, 2m, 3h)")
	cmd.Flags().BoolVarP(&flags.wait, "wait", "w", true, "wait until the deployment finishes")
	return cmd
}

// catalogItemPicker prompts the user to pick a catalog item from a list and
// returns the chosen item's name. It is an interface so tests can substitute a
// deterministic picker instead of driving the terminal.
type catalogItemPicker func(items []types.GitCatalogItem) (string, error)

// ExecuteDeploy resolves the catalog item, merges variables, triggers a deploy,
// and optionally waits for it to finish while streaming its logs. When
// catalogItemName is empty the user is prompted to pick one from an interactive
// list, matching the UX of `okteto namespace use`.
func (c *Command) ExecuteDeploy(ctx context.Context, catalogItemName string, flags *deployFlags) error {
	err := c.executeDeploy(ctx, catalogItemName, flags, promptForCatalogItem)
	analytics.TrackCatalogDeploy(err == nil, len(flags.variables) > 0)
	return err
}

func (c *Command) executeDeploy(ctx context.Context, catalogItemName string, flags *deployFlags, pick catalogItemPicker) error {
	items, err := c.okClient.Catalog().List(ctx)
	if err != nil {
		var uErr oktetoErrors.UserError
		if errors.As(err, &uErr) {
			return uErr
		}
		return fmt.Errorf("failed to list catalog items: %w", err)
	}

	if catalogItemName == "" {
		if len(items) == 0 {
			return errNoCatalogItems
		}
		catalogItemName, err = pick(items)
		if err != nil {
			return err
		}
	}

	item, ok := findCatalogItem(items, catalogItemName)
	if !ok {
		return notFoundError(catalogItemName, items)
	}

	userVars, err := parseVariableOverrides(flags.variables)
	if err != nil {
		return err
	}
	mergedVars := mergeVariables(item.Variables, userVars)

	opts := types.CatalogDeployOptions{
		CatalogItemID: item.ID,
		Name:          firstNonEmpty(flags.name, item.Name),
		Repository:    item.RepositoryURL,
		Branch:        firstNonEmpty(flags.branch, item.Branch),
		Filename:      firstNonEmpty(flags.file, item.ManifestPath),
		Namespace:     firstNonEmpty(flags.namespace, okteto.GetContext().Namespace),
		Variables:     mergedVars,
	}

	oktetoLog.Spinner(fmt.Sprintf("Deploying '%s' from the catalog...", opts.Name))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	resp, err := c.okClient.Catalog().Deploy(ctx, opts)
	if err != nil {
		var uErr oktetoErrors.UserError
		if errors.As(err, &uErr) {
			return uErr
		}
		return fmt.Errorf("failed to deploy catalog item '%s': %w", item.Name, err)
	}

	if !flags.wait {
		oktetoLog.StopSpinner()
		oktetoLog.Success("Development Environment '%s' scheduled for deployment", opts.Name)
		return nil
	}

	if err := c.waitUntilRunning(ctx, opts.Name, opts.Namespace, resp.Action, flags.timeout); err != nil {
		return fmt.Errorf("wait for '%s' to finish failed: %w", opts.Name, err)
	}

	oktetoLog.StopSpinner()
	oktetoLog.Success("Development Environment '%s' successfully deployed", opts.Name)
	return nil
}

// waitUntilRunning blocks until the deploy action finishes or CTRL+C is hit,
// concurrently streaming the deploy's logs so the user sees live progress.
func (c *Command) waitUntilRunning(ctx context.Context, name, namespace string, action *types.Action, timeout time.Duration) error {
	oktetoLog.Spinner(fmt.Sprintf("Waiting for '%s' to be deployed...", name))

	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	defer signal.Stop(stop)

	// The channel is buffered for both writers so the losing goroutine never
	// blocks after the select has resolved.
	exit := make(chan error, 2)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := c.okClient.Pipeline().WaitForActionProgressing(waitCtx, name, namespace, action.Name, timeout); err != nil {
			oktetoLog.Infof("waiting for action to progress failed: %v", err)
			exit <- err
			return
		}
		// Log streaming errors are advisory; they don't signal deploy failure.
		if err := c.okClient.Stream().PipelineLogs(waitCtx, name, namespace, action.Name, timeout); err != nil {
			oktetoLog.Warning("deploy logs cannot be streamed due to connectivity issues")
			oktetoLog.Infof("deploy logs cannot be streamed due to connectivity issues: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		exit <- c.okClient.Pipeline().WaitForActionToFinish(waitCtx, name, namespace, action.Name, timeout)
	}()

	select {
	case <-stop:
		oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
		cancel()
		wg.Wait()
		return oktetoErrors.ErrIntSig
	case err := <-exit:
		cancel()
		wg.Wait()
		return err
	}
}

// notFoundError builds a user-friendly "not found" error, with a "did you mean?"
// suggestion when a sufficiently similar catalog item name exists.
func notFoundError(requested string, items []types.GitCatalogItem) error {
	suggestion := closestName(requested, items)
	if suggestion != "" {
		return fmt.Errorf("%w: %q. Did you mean %q? Run 'okteto catalog list' to see available items",
			errCatalogItemNotFound, requested, suggestion)
	}
	return fmt.Errorf("%w: %q. Run 'okteto catalog list' to see available items",
		errCatalogItemNotFound, requested)
}

// closestName returns the item name whose Levenshtein distance from target is
// smallest, as long as it is within suggestionMaxDistance. An empty string
// means no suggestion is worth offering.
func closestName(target string, items []types.GitCatalogItem) string {
	best := ""
	bestDist := suggestionMaxDistance + 1
	for _, it := range items {
		d := levenshtein.Distance(strings.ToLower(target), strings.ToLower(it.Name), nil)
		if d < bestDist {
			bestDist = d
			best = it.Name
		}
	}
	if bestDist > suggestionMaxDistance {
		return ""
	}
	return best
}

// findCatalogItem returns the item whose name matches exactly.
func findCatalogItem(items []types.GitCatalogItem, name string) (types.GitCatalogItem, bool) {
	for _, it := range items {
		if it.Name == name {
			return it, true
		}
	}
	return types.GitCatalogItem{}, false
}

// parseVariableOverrides converts KEY=VALUE strings into Variables.
func parseVariableOverrides(raw []string) ([]types.Variable, error) {
	vars := make([]types.Variable, 0, len(raw))
	for _, v := range raw {
		kv := strings.SplitN(v, "=", varKV)
		if len(kv) != varKV || kv[0] == "" {
			return nil, fmt.Errorf("invalid variable value %q: must follow KEY=VALUE format", v)
		}
		vars = append(vars, types.Variable{Name: kv[0], Value: kv[1]})
	}
	return vars, nil
}

// mergeVariables returns catalog defaults overridden by user values, matching the UI flow.
func mergeVariables(defaults []types.GitCatalogItemVariable, overrides []types.Variable) []types.Variable {
	merged := make(map[string]string, len(defaults)+len(overrides))
	order := make([]string, 0, len(defaults)+len(overrides))
	for _, d := range defaults {
		if _, seen := merged[d.Name]; !seen {
			order = append(order, d.Name)
		}
		merged[d.Name] = d.Value
	}
	for _, o := range overrides {
		if _, seen := merged[o.Name]; !seen {
			order = append(order, o.Name)
		}
		merged[o.Name] = o.Value
	}

	out := make([]types.Variable, 0, len(order))
	for _, name := range order {
		out = append(out, types.Variable{Name: name, Value: merged[name]})
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// promptForCatalogItem shows an interactive list of catalog items, returns the
// name of the one the user picked. Items are sorted alphabetically so repeated
// runs show them in the same order. Each entry's label includes the repository
// and branch so the user can tell similar items apart at a glance.
func promptForCatalogItem(items []types.GitCatalogItem) (string, error) {
	sorted := make([]types.GitCatalogItem, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	options := make([]utils.SelectorItem, 0, len(sorted))
	for _, it := range sorted {
		options = append(options, utils.SelectorItem{
			Name:   it.Name,
			Label:  formatPickerLabel(it),
			Enable: true,
		})
	}

	selector := utils.NewOktetoSelector("Select the catalog item to deploy:", "Catalog Item")
	chosen, err := selector.AskForOptionsOkteto(options, -1)
	if err != nil {
		return "", err
	}
	return chosen, nil
}

// formatPickerLabel renders a catalog item as "<name> — <repo>@<branch>" so the
// selector shows enough context to disambiguate similarly-named items.
func formatPickerLabel(it types.GitCatalogItem) string {
	branch := it.Branch
	if branch == "" {
		branch = "default"
	}
	if it.RepositoryURL == "" {
		return it.Name
	}
	return fmt.Sprintf("%s — %s@%s", it.Name, it.RepositoryURL, branch)
}
