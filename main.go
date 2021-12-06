// Copyright 2021 The Okteto Authors
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
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/okteto/okteto/cmd"
	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/deploy"
	"github.com/okteto/okteto/cmd/destroy"
	initCMD "github.com/okteto/okteto/cmd/init"
	"github.com/okteto/okteto/cmd/namespace"
	"github.com/okteto/okteto/cmd/pipeline"
	"github.com/okteto/okteto/cmd/preview"
	"github.com/okteto/okteto/cmd/stack"
	"github.com/okteto/okteto/cmd/up"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	utilRuntime "k8s.io/apimachinery/pkg/util/runtime"

	// Load the different library for authentication
	_ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	generateFigSpec "github.com/withfig/autocomplete-tools/packages/cobra"
)

func init() {
	log.SetLevel("warn")

	// override client-go error handlers to downgrade the "logging before flag.Parse" error
	errorHandlers := []func(error){
		func(e error) {

			// Error when there's no service on the other side: an error occurred forwarding 8080 -> 8080: error forwarding port 8080 to pod df05e7e1c85d6b256df779c0b2deb417eb53a299c8581fc0945264301d8fa4b3, uid : exit status 1: 2020/03/16 02:11:09 socat[42490] E connect(5, AF=2 127.0.0.1:8080, 16): Connection refused\n
			log.Debugf("port-forward: %s", e)
		},
	}

	utilRuntime.ErrorHandlers = errorHandlers

	if bin := os.Getenv(model.OktetoBinEnvVar); bin != "" {
		model.OktetoBinImageTag = bin
		log.Infof("using %s as the bin image", bin)
	}
}

func main() {
	ctx := context.Background()
	log.Init(logrus.WarnLevel)
	var logLevel string

	if err := analytics.Init(); err != nil {
		log.Infof("error initializing okteto analytics: %s", err)
	}

	okteto.InitContextWithDeprecatedToken()

	root := &cobra.Command{
		Use:           fmt.Sprintf("%s COMMAND [ARG...]", config.GetBinaryName()),
		Short:         "Okteto - Remote Development Environments powered by Kubernetes",
		Long:          "Okteto - Remote Development Environments powered by Kubernetes",
		SilenceErrors: true,
		PersistentPreRun: func(ccmd *cobra.Command, args []string) {
			ccmd.SilenceUsage = true
			log.SetLevel(logLevel)
			log.Infof("started %s", strings.Join(os.Args, " "))

		},
		PersistentPostRun: func(ccmd *cobra.Command, args []string) {
			log.Infof("finished %s", strings.Join(os.Args, " "))
		},
	}

	root.PersistentFlags().StringVarP(&logLevel, "loglevel", "l", "warn", "amount of information outputted (debug, info, warn, error)")
	root.AddCommand(cmd.Analytics())
	root.AddCommand(cmd.Version())
	root.AddCommand(cmd.Login())
	root.AddCommand(contextCMD.Context())
	root.AddCommand(cmd.Build(ctx))
	root.AddCommand(cmd.Create(ctx))
	root.AddCommand(cmd.List(ctx))
	root.AddCommand(cmd.Delete(ctx))
	root.AddCommand(namespace.Namespace(ctx))
	root.AddCommand(pipeline.Pipeline(ctx))
	root.AddCommand(stack.Stack(ctx))
	root.AddCommand(initCMD.Init())
	root.AddCommand(up.Up())
	root.AddCommand(cmd.Down())
	root.AddCommand(cmd.Push(ctx))
	root.AddCommand(cmd.Status())
	root.AddCommand(cmd.Doctor())
	root.AddCommand(cmd.Exec())
	root.AddCommand(preview.Preview(ctx))
	root.AddCommand(cmd.Restart())
	root.AddCommand(cmd.Update())
	root.AddCommand(deploy.Deploy(ctx))
	root.AddCommand(destroy.Destroy(ctx))
	root.AddCommand(generateFigSpec.NewCmdGenFigSpec())
	err := root.Execute()

	if err != nil {
		message := err.Error()
		if len(message) > 0 {
			tmp := []rune(message)
			tmp[0] = unicode.ToUpper(tmp[0])
			message = string(tmp)
		}
		log.Fail(message)
		if uErr, ok := err.(errors.UserError); ok {
			if len(uErr.Hint) > 0 {
				log.Hint("    %s", uErr.Hint)
			}
		}
		os.Exit(1)
	}
}
