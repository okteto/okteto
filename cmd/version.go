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

package cmd

import (
	"errors"
	"fmt"
	"github.com/Masterminds/semver/v3"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/discovery"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/resolve"
	"github.com/spf13/cobra"
	"os"
)

// Version returns information about the binary
func Version() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "View the version of the okteto binary",
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/okteto-cli/#version"),
		RunE:  Show().RunE,
		//PreRun: func(ccmd *cobra.Command, args []string) {
		//// check what version the cluster is running
		//// if the version is different from the current version
		//// invoke the CLI for that version
		//currentVersion := "2.25.2"
		//clusterVersion := os.Getenv("OKTETO_CLUSTER_VERSION")
		//bin := fmt.Sprintf("/Users/andrea/.okteto/bin/%s/okteto", clusterVersion)
		//if currentVersion != clusterVersion {
		//	fmt.Printf("Calling version: '%s %s %s'\n", bin, ccmd.Name(), args)
		//	// redirect command to bin:
		//	// newArgs should contain the cmd name plus all args
		//	newArgs := []string{ccmd.Name()}
		//	newArgs = append(newArgs, args...)
		//
		//	ncmd := &exec.Cmd{
		//		Path: bin,
		//		Args: newArgs,
		//	}
		//	ncmd.Stdout = os.Stdout
		//	ncmd.Stderr = os.Stderr
		//	ncmd.Stdin = os.Stdin
		//	ncmd.Env = os.Environ()
		//	err := ncmd.Run()
		//	if err != nil {
		//		oktetoLog.Infof("error running the command: %s", err)
		//	}
		//}
		//},
	}
	cmd.AddCommand(Update())
	cmd.AddCommand(Show())
	return cmd
}

// Update checks if there is a new version available and updates it
func Update() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update Okteto CLI version",
		RunE: func(cmd *cobra.Command, args []string) error {
			currentVersion, err := semver.NewVersion(config.VersionString)
			if err != nil {
				return fmt.Errorf("could not retrieve version")
			}

			if isUpdateAvailable(currentVersion) {
				displayUpdateSteps()
			} else {
				oktetoLog.Success("The latest okteto version is already installed")
			}

			return nil
		},
	}
}

// Show shows the current Okteto CLI version
func Show() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show Okteto CLI version",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			discoveredManifestPath, err := discovery.GetOktetoManifestPath(cwd)
			if err != nil {
				if !errors.Is(err, discovery.ErrOktetoManifestNotFound) {
					return err
				}
			}
			if resolve.ShouldRedirect("", "", discoveredManifestPath) {
				return resolve.RedirectCmd(cmd, args)
			}

			oktetoLog.Printf("okteto version %s \n", config.VersionString)
			return nil
		},
	}
}
