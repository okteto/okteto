package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/okteto/okteto/cmd"
	"github.com/okteto/okteto/cmd/namespace"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
	"github.com/opentracing/opentracing-go"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.undefinedlabs.com/scopeagent"
	"go.undefinedlabs.com/scopeagent/instrumentation/process"
	"k8s.io/apimachinery/pkg/util/runtime"

	// Load the different library for authentication
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

func init() {
	// override client-go error handlers to downgrade the "logging before flag.Parse" error
	errorHandlers := []func(error){
		func(e error) {
			log.Debugf("unhandled error: %s", e)
		},
	}

	runtime.ErrorHandlers = errorHandlers
}

func main() {
	log.Init(logrus.WarnLevel)
	var logLevel string

	defer scopeagent.GlobalAgent.Stop()

	// Start a span representing this process execution, following the trace found in the environment (if available)
	span := process.StartSpan(filepath.Base(os.Args[0]))
	defer span.Finish()

	// Create a new context to be used in my application
	ctx := opentracing.ContextWithSpan(context.Background(), span)

	root := &cobra.Command{
		Use:           fmt.Sprintf("%s COMMAND [ARG...]", config.GetBinaryName()),
		Short:         "Manage cloud dev environments",
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
	root.AddCommand(cmd.Create(ctx))
	root.AddCommand(cmd.Delete())
	root.AddCommand(namespace.Namespace())
	root.AddCommand(cmd.Init())
	root.AddCommand(cmd.Up())
	root.AddCommand(cmd.Down())
	root.AddCommand(cmd.Exec())
	root.AddCommand(cmd.Restart())

	if err := root.Execute(); err != nil {
		log.Fail(err.Error())
		if uErr, ok := err.(errors.UserError); ok {
			if len(uErr.Hint) > 0 {
				log.Hint("    %s", uErr.Hint)
			}
		}

		os.Exit(1)
	}
}
