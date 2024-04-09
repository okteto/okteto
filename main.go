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

package main

import (
	"context"
	cryptoRand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/okteto/okteto/cmd"
	"github.com/okteto/okteto/cmd/build"
	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/deploy"
	"github.com/okteto/okteto/cmd/destroy"
	"github.com/okteto/okteto/cmd/kubetoken"
	"github.com/okteto/okteto/cmd/logs"
	"github.com/okteto/okteto/cmd/namespace"
	"github.com/okteto/okteto/cmd/pipeline"
	"github.com/okteto/okteto/cmd/preview"
	"github.com/okteto/okteto/cmd/registrytoken"
	"github.com/okteto/okteto/cmd/remoterun"
	"github.com/okteto/okteto/cmd/stack"
	"github.com/okteto/okteto/cmd/test"
	"github.com/okteto/okteto/cmd/up"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/insights"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	generateFigSpec "github.com/withfig/autocomplete-tools/packages/cobra"
	utilRuntime "k8s.io/apimachinery/pkg/util/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth/azure" // Load the different library for authentication
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"   // Load the different library for authentication
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"  // Load the different library for authentication
)

func init() {
	oktetoLog.SetLevel("warn") // TODO: Remove when we fully move to ioController
	var b [16]byte
	_, err := cryptoRand.Read(b[:])
	seed := int64(binary.LittleEndian.Uint64(b[:]))
	if err != nil {
		oktetoLog.Info("cannot use cryptoRead. Fallback to timestamp seed generator") // TODO: Remove when we fully move to ioController
		seed = time.Now().UnixNano()
	}
	rand.New(rand.NewSource(seed))

	// override client-go error handlers to downgrade the "logging before flag.Parse" error
	errorHandlers := []func(error){
		func(e error) {

			// Error when there's no service on the other side: an error occurred forwarding 8080 -> 8080: error forwarding port 8080 to pod df05e7e1c85d6b256df779c0b2deb417eb53a299c8581fc0945264301d8fa4b3, uid : exit status 1: 2020/03/16 02:11:09 socat[42490] E connect(5, AF=2 127.0.0.1:8080, 16): Connection refused\n
			oktetoLog.Debugf("port-forward: %s", e)
		},
	}

	utilRuntime.ErrorHandlers = errorHandlers

	if bin := os.Getenv(model.OktetoBinEnvVar); bin != "" {
		model.OktetoBinImageTag = bin
		oktetoLog.Infof("using %s as the bin image", bin)
	}
}

func main() {
	ctx := context.Background()
	ioController := io.NewIOController()
	ioController.Logger().SetLevel(io.WarnLevel)
	oktetoLog.Init(logrus.WarnLevel) // TODO: Remove when we fully move to ioController
	if registrytoken.IsRegistryCredentialHelperCommand(os.Args) {
		oktetoLog.SetOutput(os.Stderr)                  // TODO: Remove when we fully move to ioController
		oktetoLog.SetLevel(oktetoLog.InfoLevel)         // TODO: Remove when we fully move to ioController
		oktetoLog.SetOutputFormat(oktetoLog.JSONFormat) // TODO: Remove when we fully move to ioController

		ioController.Logger().SetLevel(io.InfoLevel)
		ioController.SetOutputFormat(io.JSONFormat)
	}

	var logLevel string
	var outputMode string
	var serverNameOverride string

	if err := analytics.Init(); err != nil {
		oktetoLog.Infof("error initializing okteto analytics: %s", err)
	}

	okteto.InitContextWithDeprecatedToken()

	k8sLogger := io.NewK8sLogger()

	root := &cobra.Command{
		Use:           fmt.Sprintf("%s COMMAND [ARG...]", config.GetBinaryName()),
		Short:         "Okteto - Remote Development Environments powered by Kubernetes",
		Long:          "Okteto - Remote Development Environments powered by Kubernetes",
		SilenceErrors: true,
		PersistentPreRun: func(ccmd *cobra.Command, args []string) {
			ccmd.SilenceUsage = true
			if !registrytoken.IsRegistryCredentialHelperCommand(os.Args) {
				oktetoLog.SetLevel(logLevel)          // TODO: Remove when we fully move to ioController
				oktetoLog.SetOutputFormat(outputMode) // TODO: Remove when we fully move to ioController

				ioController.Logger().SetLevel(logLevel)
				ioController.SetOutputFormat(outputMode)
			}
			okteto.SetServerNameOverride(serverNameOverride)
			ioController.Logger().Infof("started %s", strings.Join(os.Args, " "))

			if k8sLogger.IsEnabled() {
				cmdName, flags := getCurrentCmdWithUsedFlags(ccmd)
				k8sLogger.Start(config.GetOktetoHome(), cmdName, flags)
				ioController.Logger().Debugf("okteto k8s log file: %s", io.GetK8sLoggerFilePath(config.GetOktetoHome()))
			}
		},
		PersistentPostRun: func(ccmd *cobra.Command, args []string) {
			ioController.Logger().Infof("finished %s", strings.Join(os.Args, " "))
		},
	}

	root.PersistentFlags().StringVarP(&logLevel, "log-level", "l", "warn", "amount of information outputted (debug, info, warn, error)")
	root.PersistentFlags().StringVar(&outputMode, "log-output", oktetoLog.TTYFormat, "output format for logs (tty, plain, json)")

	root.PersistentFlags().StringVarP(&serverNameOverride, "server-name", "", "", "The address and port of the Okteto Ingress server")
	err := root.PersistentFlags().MarkHidden("server-name")
	if err != nil {
		ioController.Logger().Infof("error hiding server-name flag: %s", err)
	}

	okClientProvider := okteto.NewOktetoClientProvider()
	k8sClientProvider := okteto.NewK8sClientProvider()

	insights := insights.NewInsightsPublisher(k8sClientProvider, *ioController)
	at := analytics.NewAnalyticsTracker()

	root.AddCommand(cmd.Analytics())
	root.AddCommand(cmd.Version())
	root.AddCommand(cmd.Login())

	root.AddCommand(contextCMD.Context(okClientProvider))
	root.AddCommand(cmd.Kubeconfig(okClientProvider))

	root.AddCommand(kubetoken.NewKubetokenCmd().Cmd())
	root.AddCommand(registrytoken.RegistryToken(ctx))

	root.AddCommand(build.Build(ctx, ioController, at, insights, k8sLogger))

	root.AddCommand(namespace.Namespace(ctx, k8sLogger))
	root.AddCommand(cmd.Init(at, insights, ioController))
	root.AddCommand(up.Up(at, insights, ioController, k8sLogger))
	root.AddCommand(cmd.Down(k8sLogger))
	root.AddCommand(cmd.Status())
	root.AddCommand(cmd.Doctor(k8sLogger))
	root.AddCommand(cmd.Exec(k8sLogger))
	root.AddCommand(preview.Preview(ctx))
	root.AddCommand(cmd.Restart())
	root.AddCommand(cmd.UpdateDeprecated())
	root.AddCommand(deploy.Deploy(ctx, at, insights, ioController, k8sLogger))
	root.AddCommand(destroy.Destroy(ctx, at, insights, ioController, k8sLogger))
	root.AddCommand(deploy.Endpoints(ctx, k8sLogger))
	root.AddCommand(logs.Logs(ctx, k8sLogger))
	root.AddCommand(generateFigSpec.NewCmdGenFigSpec())
	root.AddCommand(remoterun.RemoteRun(ctx, k8sLogger))
	root.AddCommand(test.Test(ctx, ioController, k8sLogger, at))

	// deprecated
	root.AddCommand(cmd.Create(ctx))
	root.AddCommand(cmd.List(ctx))
	root.AddCommand(cmd.Delete(ctx))
	root.AddCommand(stack.Stack(ctx, at, insights, ioController))
	root.AddCommand(cmd.Push(ctx))
	root.AddCommand(pipeline.Pipeline(ctx))

	err = root.Execute()

	if err != nil {
		message := err.Error()
		if len(message) > 0 {
			tmp := []rune(message)
			tmp[0] = unicode.ToUpper(tmp[0])
			message = string(tmp)
		}
		oktetoLog.Fail(message) // TODO: Change to use ioController  when we fully move to ioController
		if uErr, ok := err.(oktetoErrors.UserError); ok {
			if len(uErr.Hint) > 0 {
				oktetoLog.Hint("    %s", uErr.Hint)
			}
		}
		os.Exit(1)
	}
}

func getCurrentCmdWithUsedFlags(cmd *cobra.Command) (string, string) {
	var flags []string
	cmd.Flags().Visit(func(f *pflag.Flag) {
		if f.Changed {
			flags = append(flags, fmt.Sprintf("--%s=%s", f.Name, f.Value))
		}
	})

	return cmd.Name(), strings.Join(flags, " ")
}
