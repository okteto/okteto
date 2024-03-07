package resolve

import (
	"fmt"
	"github.com/okteto/okteto/pkg/config"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
)

func ShouldRedirect() bool {
	currentVersion := config.VersionString
	clusterVersion := os.Getenv("OKTETO_CLUSTER_VERSION")

	return currentVersion != clusterVersion
}

func RedirectCmd(cmd *cobra.Command, args []string) error {
	clusterVersion := os.Getenv("OKTETO_CLUSTER_VERSION")
	bin := fmt.Sprintf("/Users/andrea/.okteto/bin/%s/okteto", clusterVersion)

	var newArgs []string
	if cmd.Parent() != nil {
		newArgs = append(newArgs, cmd.Parent().Name())
	} else {
		newArgs = append(newArgs, cmd.Name())
	}
	newArgs = append(newArgs, args...)

	ncmd := exec.Command(bin, newArgs...)

	fmt.Printf("Executing: '%s %s'\n", bin, newArgs)
	ncmd.Stdout = os.Stdout
	ncmd.Stderr = os.Stderr
	ncmd.Stdin = os.Stdin
	ncmd.Env = os.Environ()
	return ncmd.Run()
}
