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

// Package divertrouter is the in-cluster entry point of the divert data plane. It is not a
// command developers run: `okteto divert up` deploys it as the workload sitting in front of
// a diverted service.
package divertrouter

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/okteto/okteto/pkg/divert/router"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/spf13/cobra"
)

// logLevelFlag is the CLI-wide log level flag, declared on the root command.
const logLevelFlag = "log-level"

// DivertRouter returns the hidden command that serves the divert router. It is configured
// entirely through the environment, since it runs as a container spec written by
// `okteto divert up` rather than as a command anyone types.
func DivertRouter(ioCtrl *io.Controller) *cobra.Command {
	return &cobra.Command{
		Use:          "divert-router",
		Short:        "Serve the divert router in front of a diverted service. Runs inside the cluster",
		Hidden:       true,
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		// Runs after the root command's PersistentPreRun has applied the CLI-wide level.
		PreRun: func(cmd *cobra.Command, _ []string) {
			if shouldRaiseLogLevel(cmd) {
				ioCtrl.Logger().SetLevel(io.InfoLevel)
			}
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return Run(cmd.Context(), os.Getenv, ioCtrl.Logger())
		},
	}
}

// shouldRaiseLogLevel reports whether the router should log at info rather than inherit the
// CLI default.
//
// The CLI defaults to warn, which is right for a command a developer runs and wrong for a
// long-running proxy: every line the router emits is an Infof, so at warn it serves traffic
// in total silence. Its request log is the only way to answer "why didn't my divert work",
// so it defaults to info here instead of relying on whoever writes the pod spec to
// remember a flag. An explicit --log-level still wins.
func shouldRaiseLogLevel(cmd *cobra.Command) bool {
	flag := cmd.Root().PersistentFlags().Lookup(logLevelFlag)
	return flag == nil || !flag.Changed
}

// routeTable picks how the router learns its routes.
//
// A mounted directory is the normal case: `okteto divert up` keeps a ConfigMap of routing
// keys, Kubernetes materialises it into the pod, and the router re-reads it. That costs the
// router no API access at all, so it needs no ServiceAccount and no RBAC. The static
// environment variable remains for a router driven without a ConfigMap.
func routeTable(ctx context.Context, cfg *router.Config, logger router.Logger) router.Table {
	if cfg.RoutesDir == "" {
		logger.Infof("reading routes from the %s environment variable", router.EnvRoutes)
		return router.NewStaticTable(cfg.Service, cfg.Routes, logger)
	}

	logger.Infof("reading routes from %s, reloading every %s", cfg.RoutesDir, router.DefaultReloadInterval)
	table := router.NewFileTable(cfg.Service, cfg.RoutesDir, logger)
	go table.Watch(ctx, router.DefaultReloadInterval)

	return table
}

// Run loads the configuration from the environment and serves until the context is
// cancelled or the process is signalled.
func Run(ctx context.Context, getenv func(string) string, logger router.Logger) error {
	cfg, err := router.LoadConfig(getenv)
	if err != nil {
		return fmt.Errorf("invalid divert router configuration: %w", err)
	}

	logger.Infof(
		"starting divert router for service %q in namespace %q: baseline %s, %s",
		cfg.Service, cfg.SharedNamespace, cfg.BaselineHost, describePorts(cfg.Ports),
	)

	// One route table shared by every port: the routing key selects a namespace, not a port.
	table := routeTable(ctx, cfg, logger)

	handlers := make(map[int]http.Handler, len(cfg.Ports))
	for _, port := range cfg.Ports {
		handlers[port.Listen] = router.New(cfg, port, table, logger)
	}

	return serve(ctx, cfg, handlers, logger)
}

func describePorts(ports []router.PortConfig) string {
	described := make([]string, 0, len(ports))
	for _, port := range ports {
		described = append(described, fmt.Sprintf("listening on %d forwarding to %d", port.Listen, port.Service))
	}

	return strings.Join(described, ", ")
}
