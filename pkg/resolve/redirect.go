package resolve

import (
	"fmt"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
)

type Redirect struct {
	currentVersion string
	clusterVersion string
}

func NewRedirect(currentVersion string) *Redirect {
	// POC
	if currentVersion == "" {
		currentVersion = "2.25.2"
	}
	return &Redirect{
		currentVersion: currentVersion,
	}
}

func (r *Redirect) ShouldRedirect(flagContext, manifestFilePath string) bool {
	if r.clusterVersion == "" {
		r.clusterVersion = r.getClusterVersion(flagContext, manifestFilePath)
	}

	return r.currentVersion != r.clusterVersion
}

func (r *Redirect) getClusterVersion(flagContext, manifestFilePath string) string {
	contextsToVersion := map[string]string{
		"https://product.okteto.dev":                   "2.25.2",
		"https://okteto.andreafalzetti.dev.okteto.net": "2.22.0",
		"http://staging.okteto.dev":                    "2.23.0",
	}

	ctxStr, err := getContext(flagContext, manifestFilePath)
	if err != nil {
		fmt.Println("error getting context: ", err)
	}

	fmt.Printf("Context: %s\n", ctxStr)
	clusterVersion, _ := contextsToVersion[ctxStr]

	return clusterVersion
}

func getContext(flagContext, manifestFilePath string) (string, error) {
	if flagContext != "" {
		return flagContext, nil
	}
	// get context from manifest
	ctxResource, err := model.GetContextResource(manifestFilePath)
	if err != nil {
		return "", err
	}

	if ctxResource.Context != "" {
		return ctxResource.Context, nil
	}

	ctxStore := okteto.GetContextStore()

	return ctxStore.CurrentContext, nil
}

func (r *Redirect) RedirectCmd(cmd *cobra.Command, args []string) error {
	bin := fmt.Sprintf("/Users/andrea/.okteto/bin/%s/okteto", r.clusterVersion)

	var newArgs []string
	if cmd.Parent() != nil && cmd.Parent().Name() != "okteto" {
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
