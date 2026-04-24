// Copyright 2025 The Okteto Authors
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
	"net/url"
	"strings"

	contextCMD "github.com/okteto/okteto/cmd/context"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"
)

const adminCatalogPath = "/admin/catalog/new"

// errNotAdmin is returned when a non-admin user runs `okteto catalog add`.
// The mutations behind the form are admin-only on the backend, so there's no
// point opening the UI for a user who can't submit it.
var errNotAdmin = oktetoErrors.UserError{
	E:    errors.New("adding a Catalog item requires administrator privileges"),
	Hint: "Ask your Okteto administrator to add the item, or log in with an admin account.",
}

// browserOpener is injected for testing so we do not spawn a browser in unit tests.
type browserOpener func(url string) error

// Add returns the `okteto catalog add` cobra command. It opens the Okteto UI on
// the "new Catalog item" page. Catalog item creation is admin-only, so this
// command performs a pre-flight admin check before launching the browser.
func Add(ctx context.Context) *cobra.Command {
	var k8sContext string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Open the Okteto UI to add a new Catalog item (admin only)",
		Long: `Open the Okteto web UI on the "new Catalog item" page.

Catalog items are managed from the Okteto UI and can only be created by Okteto
administrators. This command checks your admin status and, if authorised,
launches the browser on the admin page.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.Options{Context: k8sContext, Show: true}); err != nil {
				return err
			}
			if !okteto.IsOkteto() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}

			c, err := NewCommand()
			if err != nil {
				return err
			}
			return c.ExecuteAdd(ctx, open.Start)
		},
	}
	cmd.Flags().StringVarP(&k8sContext, "context", "c", "", "overwrite the current Okteto Context")
	return cmd
}

// ExecuteAdd runs the admin pre-flight check and opens the browser on the
// Catalog admin page. Passing the browserOpener in keeps the function unit
// testable without launching a real browser.
func (c *Command) ExecuteAdd(ctx context.Context, openBrowser browserOpener) error {
	if err := ensureAdmin(ctx, c.okClient.User()); err != nil {
		return err
	}
	return runAdd(okteto.GetContext().Name, openBrowser)
}

// ensureAdmin checks admin status via the Okteto API. A non-admin user is
// rejected with a clear error. A generic query failure fails closed so auth or
// configuration issues surface here rather than confusing the user at the UI
// submission step. The single exception is a very old server that does not
// expose `super` on the `me` type; in that case we defer to the UI since the
// backend still enforces authorization on submission.
func ensureAdmin(ctx context.Context, user types.UserInterface) error {
	admin, err := user.IsAdmin(ctx)
	if err != nil {
		if isSuperFieldMissingErr(err) {
			oktetoLog.Warning("This Okteto instance does not expose admin status; the Okteto UI will reject the submission if you are not an administrator.")
			return nil
		}
		return fmt.Errorf("could not verify administrator privileges: %w", err)
	}
	if !admin {
		return errNotAdmin
	}
	return nil
}

// isSuperFieldMissingErr reports whether the error indicates the server does
// not know about the `super` field — the one narrow case where we fall back
// to the UI's admin gate instead of blocking the user.
func isSuperFieldMissingErr(err error) bool {
	return strings.Contains(err.Error(), `Cannot query field "super"`)
}

// runAdd builds the admin catalog URL from the current context and opens it.
func runAdd(contextURL string, openBrowser browserOpener) error {
	target, err := buildAdminCatalogURL(contextURL)
	if err != nil {
		return err
	}

	oktetoLog.Information("Opening the Okteto Catalog in your browser to add a new item")
	if err := openBrowser(target); err != nil {
		oktetoLog.Warning("Could not open the browser automatically: %s", err)
		oktetoLog.Println(fmt.Sprintf("Open this URL manually: %s", target))
		return nil
	}
	oktetoLog.Println(fmt.Sprintf("If the page does not open, visit: %s", target))
	return nil
}

// buildAdminCatalogURL appends the admin catalog path to the Okteto context
// URL. When the context URL has no scheme we prepend "https://" *before*
// parsing so hosts with ports or sub-paths (e.g. "example.com:8443",
// "example.com/okteto") round-trip cleanly — letting url.Parse do the splitting.
func buildAdminCatalogURL(contextURL string) (string, error) {
	if contextURL == "" {
		return "", fmt.Errorf("current Okteto Context has no URL")
	}
	trimmed := strings.TrimRight(contextURL, "/")
	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}
	u, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid Okteto Context URL %q: %w", contextURL, err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("invalid Okteto Context URL %q: missing host", contextURL)
	}
	u.Path = strings.TrimRight(u.Path, "/") + adminCatalogPath
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}
