// Copyright 2022 The Okteto Authors
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

package logs

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/devenvironment"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
	"github.com/stern/stern/stern"
)

// LogsOptions options for logs command
type LogsOptions struct {
	All          bool
	ManifestPath string
	Namespace    string
	Context      string
	exclude      string
	Watch        bool
	Include      string
	Since        time.Duration
	Tail         int64
	Timestamps   bool
	Name         string
}

func Logs(ctx context.Context) *cobra.Command {
	options := &LogsOptions{}

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Fetch the logs of your development environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			manifest, err := contextCMD.LoadManifestWithContext(ctx, contextCMD.ManifestOptions{Filename: options.ManifestPath, Namespace: options.Namespace, K8sContext: options.Context})
			if err != nil {
				return err
			}
			wd, err := os.Getwd()
			if err != nil {
				return err
			}

			// call to get dev environment name
			if options.Name != "" {
				manifest.Name = options.Name
			}
			if manifest.Name == "" {
				c, _, err := okteto.NewK8sClientProvider().Provide(okteto.Context().Cfg)
				if err != nil {
					return err
				}
				inferer := devenvironment.NewNameInferer(c)
				manifest.Name = inferer.InferName(ctx, wd, okteto.Context().Namespace, options.ManifestPath)
			}

			if len(args) > 0 {
				options.Include = args[0]
			} else {
				options.Include = ".*"
			}

			tmpKubeconfigFile := GetTempKubeConfigFile(manifest.Name)
			if err := kubeconfig.Write(okteto.Context().Cfg, tmpKubeconfigFile); err != nil {
				return err
			}
			defer os.Remove(tmpKubeconfigFile)
			c, err := getSternConfig(manifest, options, tmpKubeconfigFile)
			if err != nil {
				return errors.UserError{
					E: fmt.Errorf("invalid log configuration: %w", err),
				}
			}

			go func() {
				sigint := make(chan os.Signal, 1)
				signal.Notify(sigint, syscall.SIGTERM, syscall.SIGINT)
				<-sigint
				cancel()
			}()

			err = stern.Run(ctx, c)

			analytics.TrackLogs(err == nil, options.All)

			if err != nil {
				return errors.UserError{
					E: fmt.Errorf("failed to get logs: %w", err),
				}
			}
			return nil
		},
		Args: utils.MaximumNArgsAccepted(1, "https://www.okteto.com/docs/reference/cli/#logs"),
	}

	cmd.Flags().BoolVarP(&options.All, "all", "a", false, "fetch logs from the whole namespace")
	cmd.Flags().StringVarP(&options.ManifestPath, "file", "f", "", "path to the manifest file")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "the namespace to use to fetch the logs (defaults to the current okteto namespace)")
	cmd.Flags().StringVarP(&options.Context, "context", "c", "", "the context to use to fetch the logs")
	cmd.Flags().StringVarP(&options.exclude, "exclude", "e", "", "exclude by service name (regular expression)")
	cmd.Flags().BoolVarP(&options.Watch, "watch", "w", false, "watch the log output")
	cmd.Flags().DurationVarP(&options.Since, "since", "s", 48*time.Hour, "return logs newer than a relative duration like 5s, 2m, or 3h")
	cmd.Flags().Int64Var(&options.Tail, "tail", 100, "the number of lines from the end of the logs to show")
	cmd.Flags().BoolVarP(&options.Timestamps, "timestamps", "t", false, "print timestamps")
	cmd.Flags().StringVar(&options.Name, "name", "", "develpment environment name")

	return cmd
}

// GetTempKubeConfigFile returns where the temp kubeConfigFile for deploy should be stored
func GetTempKubeConfigFile(name string) string {
	tempKubeConfigTemplate := fmt.Sprintf("kubeconfig-logs-%s-%d", name, time.Now().UnixMilli())
	return filepath.Join(config.GetOktetoHome(), tempKubeConfigTemplate)
}
