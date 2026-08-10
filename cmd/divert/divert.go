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

// Package divert implements `okteto divert`, which puts a router in front of a service in a
// shared namespace so that requests carrying a routing header reach a developer's own copy
// of that service while everything else keeps hitting the shared one.
//
// # Required permissions
//
// Both subcommands act entirely within the shared namespace, and need nothing outside it —
// no cluster-scoped read, no access to other developers' namespaces:
//
//	apiVersion: rbac.authorization.k8s.io/v1
//	kind: Role
//	metadata:
//	  name: okteto-divert
//	  namespace: staging          # the shared namespace
//	rules:
//	  - apiGroups: [""]
//	    resources: ["services"]
//	    verbs: ["get", "list", "create", "update", "delete"]
//	  - apiGroups: ["apps"]
//	    resources: ["deployments"]
//	    verbs: ["get", "list", "create", "delete", "patch"]
//	  - apiGroups: [""]
//	    resources: ["configmaps"]
//	    verbs: ["get", "list", "create", "patch", "delete"]
//	  - apiGroups: [""]
//	    resources: ["pods"]
//	    verbs: ["list"]
//	  - apiGroups: ["discovery.k8s.io"]
//	    resources: ["endpointslices"]
//	    verbs: ["list"]
//	  - apiGroups: ["policy"]
//	    resources: ["poddisruptionbudgets"]
//	    verbs: ["create", "list", "delete"]
//
// `list` on services and deployments is what teardown uses to find leftovers by label after
// an interrupted bring-up, and `update` is the selector swap itself. Pods and endpointslices
// are read to confirm the swap has actually taken effect before bring-up reports success.
// Configmaps hold the route table, one entry per routing key, mounted into the router so
// that it needs no API access of its own. `patch` on deployments is the rolling restart of
// the diverted workload, which is what makes callers holding an older connection reconnect
// through the router; `--no-restart` skips it if that permission is not available.
//
// The disruption budget stops a node drain taking every router replica at once. It is the
// only optional permission here: without it bring-up warns and carries on.
package divert

import (
	"context"
	"fmt"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/pkg/divert/swap"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

// Divert groups the divert subcommands.
func Divert(ctx context.Context, ioCtrl *io.Controller) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "divert",
		Short:        "Route requests carrying a routing header to your own copy of a shared service",
		SilenceUsage: true,
	}

	cmd.AddCommand(Up(ctx, ioCtrl))
	cmd.AddCommand(Down(ctx, ioCtrl))

	return cmd
}

// setupClient resolves the current context and returns a swap client for the cluster it
// points at. Every divert subcommand starts here.
//
// There is deliberately no `okteto.IsOkteto()` check. Unlike the manifest-driven divert
// drivers, nothing on this path talks to the Okteto backend or to an Okteto CRD: it is
// Services and Deployments in a single namespace, and the router image falls back to a
// public one. Gating it would only block clusters it works on, and being evaluable against
// any cluster with nothing but the CLI is the point of the mesh-free approach.
func setupClient(ctx context.Context, ioCtrl *io.Controller) (*swap.Client, error) {
	if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.Options{}); err != nil {
		return nil, err
	}

	k8sClient, err := k8sClientFor(okteto.GetContext())
	if err != nil {
		return nil, err
	}

	return swap.NewClient(k8sClient, ioCtrl.Logger()), nil
}

func k8sClientFor(okCtx *okteto.Context) (kubernetes.Interface, error) {
	c, _, err := okteto.NewK8sClientProvider().Provide(okCtx.Cfg)
	if err != nil {
		return nil, fmt.Errorf("error creating kubernetes client: %w", err)
	}

	return c, nil
}
