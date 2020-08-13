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

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/okteto/okteto/cmd"
	initCMD "github.com/okteto/okteto/cmd/init"
	"github.com/okteto/okteto/cmd/namespace"
	"github.com/okteto/okteto/cmd/stack"
	"github.com/okteto/okteto/cmd/up"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/runtime"

	// Load the different library for authentication
	_ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

func init() {
	// override client-go error handlers to downgrade the "logging before flag.Parse" error
	errorHandlers := []func(error){
		func(e error) {

			// Error when there's no service on the other side: an error occurred forwarding 8080 -> 8080: error forwarding port 8080 to pod df05e7e1c85d6b256df779c0b2deb417eb53a299c8581fc0945264301d8fa4b3, uid : exit status 1: 2020/03/16 02:11:09 socat[42490] E connect(5, AF=2 127.0.0.1:8080, 16): Connection refused\n
			log.Debugf("port-forward: %s", e)
		},
	}

	runtime.ErrorHandlers = errorHandlers
}

func main() {
	ctx := context.Background()
	log.Init(logrus.WarnLevel, config.GetOktetoHome(), config.VersionString)
	log.Info("start")
	var logLevel string

	root := &cobra.Command{
		Use:           fmt.Sprintf("%s COMMAND [ARG...]", config.GetBinaryName()),
		Short:         "Manage development containers",
		SilenceErrors: true,
		PersistentPreRun: func(ccmd *cobra.Command, args []string) {
			log.SetLevel(logLevel)
			ccmd.SilenceUsage = true
		},
	}

	root.PersistentFlags().StringVarP(&logLevel, "loglevel", "l", "warn", "amount of information outputted (debug, info, warn, error)")
	root.AddCommand(cmd.Analytics())
	root.AddCommand(cmd.Version())
	root.AddCommand(cmd.Login())
	root.AddCommand(cmd.Build(ctx))
	root.AddCommand(cmd.Create(ctx))
	root.AddCommand(cmd.Delete(ctx))
	root.AddCommand(namespace.Namespace(ctx))
	root.AddCommand(stack.Stack(ctx))
	root.AddCommand(initCMD.Init())
	root.AddCommand(up.Up())
	root.AddCommand(cmd.Down())
	root.AddCommand(cmd.Push(ctx))
	root.AddCommand(cmd.Status())
	root.AddCommand(cmd.Doctor())
	root.AddCommand(cmd.Exec())
	root.AddCommand(cmd.Restart())

	err := root.Execute()

	if err != nil {
		log.Fail(err.Error())
		if uErr, ok := err.(errors.UserError); ok {
			if len(uErr.Hint) > 0 {
				log.Hint("    %s", uErr.Hint)
			}
		}

		os.Exit(1)
	}
}
