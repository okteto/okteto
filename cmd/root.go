package cmd

import (
	"fmt"
	"strings"
	"sync"

	"github.com/cloudnativedevelopment/cnd/pkg/analytics"
	"github.com/cloudnativedevelopment/cnd/pkg/config"
	"github.com/cloudnativedevelopment/cnd/pkg/errors"
	"github.com/cloudnativedevelopment/cnd/pkg/log"
	"github.com/spf13/cobra"
	runtime "k8s.io/apimachinery/pkg/util/runtime"
	kExec "k8s.io/client-go/util/exec"

	// Load the GCP library for authentication
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

type cliConfig struct {
	logLevel string
	actionID string
}

type commandFunc func() *cobra.Command

var (
	c = &cliConfig{
		actionID: analytics.NewActionID(),
	}

	analyticsWG = sync.WaitGroup{}
	commandsFN  = []commandFunc{
		Up,
		Exec,
		Down,
		Version,
		List,
		Run,
		Create,
		Analytics,
	}

	exitInformation = ""
)

// Execute runs the root command
func Execute() int {
	root := &cobra.Command{
		Use:           fmt.Sprintf("%s COMMAND [ARG...]", config.GetBinaryName()),
		Short:         "Manage cloud native environments",
		SilenceErrors: true,
		PersistentPreRun: func(ccmd *cobra.Command, args []string) {
			log.SetLevel(c.logLevel)
			ccmd.SilenceUsage = true
		},
	}

	root.PersistentFlags().StringVarP(&c.logLevel, "loglevel", "l", "warn", "amount of information outputted (debug, info, warn, error)")

	for _, fn := range commandsFN {
		root.AddCommand(fn())
	}

	// override client-go error handlers to downgrade the "logging before flag.Parse" error
	errorHandlers := []func(error){
		func(e error) {
			log.Debugf("unhandled error: %s", e)
		},
	}

	runtime.ErrorHandlers = errorHandlers

	exitCode := 0
	if err := root.Execute(); err != nil {
		errorMessage := ""
		if err == errors.ErrNotDevDeployment {
			exitCode = 137
			errorMessage = err.Error()
		} else if cerr, ok := err.(kExec.CodeExitError); ok {
			exitCode = cerr.ExitStatus()
			errorMessage = "Command failed"
		} else {
			errorMessage = err.Error()
			exitCode = 1
		}

		fmt.Printf("%s %s", log.ErrorSymbol, log.RedString(upperCaseString(errorMessage)))
		fmt.Println()
		if len(exitInformation) > 0 {
			fmt.Printf("%s %s", log.BlueString("Hint:"), exitInformation)
			fmt.Println()
		}
	}

	analytics.Wait()
	return exitCode
}

func addDevPathFlag(cmd *cobra.Command, devPath *string, value string) {
	cmd.Flags().StringVarP(devPath, "file", "f", value, "path to the manifest file")
}

func addNamespaceFlag(cmd *cobra.Command, namespace *string) {
	cmd.Flags().StringVarP(namespace, "namespace", "n", "", "kubernetes namespace to use (defaults to the current kube config namespace)")
}

// GetActionID returns the actionID used to correlate different actions in the same command
func GetActionID() string {
	return c.actionID
}

// Register registers a new command with cnd's root command
func Register(fn commandFunc) {
	commandsFN = append(commandsFN, fn)
}

func upperCaseString(s string) string {
	if len(s) == 0 {
		return ""
	}

	u := strings.ToUpper(string(s[0]))

	if len(s) == 1 {
		return u
	}

	return u + string(s[1:])
}
