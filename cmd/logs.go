// Copyright 2020 The Okteto Authors
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

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/model"

	k8Client "github.com/okteto/okteto/pkg/k8s/client"

	"github.com/spf13/cobra"
)

//Logs return the logs of the pods running as okteto services
func Logs() *cobra.Command {
	var devPath string
	var namespace string

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Shows the logs of the pods running as okteto services",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			dev, err := loadDev(devPath)
			if err != nil {
				return err
			}
			if err := dev.UpdateNamespace(namespace); err != nil {
				return err
			}
			err = executeLogs(ctx, dev, args)
			analytics.TrackLogs(err == nil)
			return err
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", defaultManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the exec command is executed")

	return cmd
}

func executeLogs(ctx context.Context, dev *model.Dev, args []string) error {
	client, _, namespace, err := k8Client.GetLocal()
	if err != nil {
		return err
	}

	if dev.Namespace == "" {
		dev.Namespace = namespace
	}

	logLines, err := pods.LogsFromServices(dev, client)
	if err != nil {
		return err
	}

	for _, logLine := range logLines {
		fmt.Printf("%s %s:%s %s\n", logLine.Timestamp.Format("2006-01-02 15:04:05"), logLine.Pod, logLine.Container, logLine.Line)
	}
	return nil
}
