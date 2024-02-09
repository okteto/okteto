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
	"context"
	"fmt"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/spf13/cobra"
)

// execFlags is the input of the user to exec command
type remoteManifestFlags struct {
	manifestPath string
	devname      string
	namespace    string
	k8sContext   string
}

// GetManifest executes a command on the CND container
func GetManifest(k8sLogger *io.K8sLogger) *cobra.Command {
	remoteManifestFlags := &remoteManifestFlags{}

	cmd := &cobra.Command{
		Use:   "get-manifest",
		Short: "Get the okteto manifest from remote",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			manifestOpts := contextCMD.ManifestOptions{Filename: remoteManifestFlags.manifestPath, DevName: remoteManifestFlags.devname, Namespace: remoteManifestFlags.namespace, K8sContext: remoteManifestFlags.k8sContext, K8sLogger: k8sLogger}
			manifest, err := contextCMD.LoadManifestWithContext(ctx, manifestOpts)
			if err != nil {
				return err
			}
			fmt.Println(string(manifest.Manifest))

			return nil
		},
	}

	cmd.Flags().StringVarP(&remoteManifestFlags.manifestPath, "file", "f", utils.DefaultManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&remoteManifestFlags.devname, "devname", "", "", "dev environment name")
	cmd.Flags().StringVarP(&remoteManifestFlags.namespace, "namespace", "n", "", "namespace where the exec command is executed")
	cmd.Flags().StringVarP(&remoteManifestFlags.k8sContext, "context", "c", "", "context where the exec command is executed")

	return cmd
}
