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

package kubetoken

import (
	"context"
	"encoding/json"
	"fmt"

	contextCMD "github.com/okteto/okteto/cmd/context"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// Flags represents the flags available for kubetoken
type Flags struct {
	Namespace string
	Context   string
}

// oktetoClientProvider provides an okteto client ready to use or fail
type oktetoClientProvider interface {
	Provide(...okteto.Option) (types.OktetoInterface, error)
}

// k8sClientProvider provides a kubernetes client ready to use or fail
type k8sClientProvider interface {
	Provide(clientApiConfig *clientcmdapi.Config) (kubernetes.Interface, *rest.Config, error)
}

// oktetoCtxCmdRunner runs the okteto context command
type oktetoCtxCmdRunner interface {
	Run(ctx context.Context, ctxOptions *contextCMD.Options) error
}

type Serializer struct{}

func (*Serializer) ToJson(kubetoken types.KubeTokenResponse) (string, error) {
	bytes, err := json.MarshalIndent(kubetoken, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

type initCtxOptsFunc func(string, string) *contextCMD.Options

// Cmd generates a kubernetes token for a given namespace
type Cmd struct {
	k8sClientProvider    k8sClientProvider
	oktetoClientProvider oktetoClientProvider
	ctxStore             *okteto.ContextStore
	oktetoCtxCmdRunner   oktetoCtxCmdRunner
	serializer           *Serializer
	initCtxFunc          initCtxOptsFunc
}

// Options represents the options for kubetoken
type Options struct {
	oktetoClientProvider oktetoClientProvider
	k8sClientProvider    k8sClientProvider
	ctxStore             *okteto.ContextStore
	oktetoCtxCmdRunner   oktetoCtxCmdRunner
	serializer           *Serializer
	getCtxResource       initCtxOptsFunc
}

func defaultKubetokenOptions() *Options {
	ctxStore := okteto.GetContextStore()
	return &Options{
		oktetoClientProvider: okteto.NewOktetoClientProvider(),
		k8sClientProvider:    okteto.NewK8sClientProvider(),
		oktetoCtxCmdRunner:   contextCMD.NewContextCommand(),
		ctxStore:             ctxStore,
		serializer:           &Serializer{},
		getCtxResource:       getCtxResource,
	}
}

type kubetokenOption func(*Options)

// NewKubetokenCmd returns a new cobra command
func NewKubetokenCmd(optFunc ...kubetokenOption) *Cmd {
	opts := defaultKubetokenOptions()
	for _, o := range optFunc {
		o(opts)
	}
	return &Cmd{
		oktetoClientProvider: opts.oktetoClientProvider,
		serializer:           opts.serializer,
		k8sClientProvider:    opts.k8sClientProvider,
		ctxStore:             opts.ctxStore,
		oktetoCtxCmdRunner:   opts.oktetoCtxCmdRunner,
		initCtxFunc:          getCtxResource,
	}
}

func (kc *Cmd) Cmd() *cobra.Command {
	var namespace string
	var contextName string

	cmd := &cobra.Command{
		Use:   "kubetoken",
		Short: "Print Kubernetes cluster credentials in ExecCredential format.",
		Long: `Print Kubernetes cluster credentials in ExecCredential format.
You can find more information on 'ExecCredential' and 'client side authentication' at (https://kubernetes.io/docs/reference/config-api/client-authentication.v1/) and  https://kubernetes.io/docs/reference/access-authn-authz/authentication/#client-go-credential-plugins`,
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			flags := Flags{
				Namespace: namespace,
				Context:   contextName,
			}
			return kc.Run(ctx, flags)
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "okteto context's namespace")
	cmd.Flags().StringVarP(&contextName, "context", "c", "", "okteto context's name")
	return cmd
}

// Run executes the kubetoken command
func (kc *Cmd) Run(ctx context.Context, flags Flags) error {
	oktetoLog.SetOutputFormat("silent")
	err := newPreReqValidator(
		withCtxName(flags.Context),
		withNamespace(flags.Namespace),
		withK8sClientProvider(kc.k8sClientProvider),
		withOktetoClientProvider(kc.oktetoClientProvider),
		withContextStore(kc.ctxStore),
		withInitContextFunc(kc.initCtxFunc),
	).validate(ctx)
	if err != nil {
		return fmt.Errorf("dynamic kubernetes token cannot be requested: %w", err)
	}

	err = kc.oktetoCtxCmdRunner.Run(ctx, &contextCMD.Options{
		Context:   flags.Context,
		Namespace: flags.Namespace,
	})
	if err != nil {
		return err
	}

	ctxResource := kc.initCtxFunc(flags.Context, flags.Namespace)
	c, err := kc.oktetoClientProvider.Provide(
		okteto.WithCtxName(ctxResource.Context),
		okteto.WithToken(ctxResource.Token),
	)
	if err != nil {
		return fmt.Errorf("failed to create okteto client: %w", err)
	}

	out, err := c.Kubetoken().GetKubeToken(ctxResource.Context, ctxResource.Namespace)
	if err != nil {
		return fmt.Errorf("failed to get the kubetoken: %w", err)
	}

	jsonStr, err := kc.serializer.ToJson(out)
	if err != nil {
		return fmt.Errorf("failed to marshal KubeTokenResponse: %w", err)
	}

	oktetoLog.SetOutputFormat("tty")
	oktetoLog.Print(jsonStr)
	return nil
}
