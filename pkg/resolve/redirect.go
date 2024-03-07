package resolve

import (
	"context"
	"fmt"
	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
)

func ShouldRedirect(flagContext, flagNamespace, manifestFilePath string) bool {
	currentVersion := config.VersionString
	clusterVersion := os.Getenv("OKTETO_CLUSTER_VERSION")

	fmt.Printf("Get context: (context=%s) (namespace=%s) (manifest=%s)\n", flagContext, flagNamespace, manifestFilePath)
	ctxStr, err := getContext(flagContext, flagNamespace, manifestFilePath)
	if err != nil {
		panic(err)
	}

	if clusterVersion == "" {
		return false
	}

	fmt.Println("CONTEXT:", ctxStr)

	return currentVersion != clusterVersion
}

func getContext(flagContext, flagNamespace, manifestFilePath string) (string, error) {
	ctxResource, err := model.GetContextResource(manifestFilePath)
	fmt.Println(ctxResource.Context, err)

	ctx := context.Background()
	// Loads, updates and uses the context from path. If not found, it creates and uses a new context
	if err := contextCMD.LoadContextFromPath(ctx, flagNamespace, flagContext, manifestFilePath, contextCMD.Options{Show: true}); err != nil {
		if err.Error() == fmt.Errorf(oktetoErrors.ErrNotLogged, okteto.CloudURL).Error() {
			return "", err
		}
		if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.Options{Namespace: flagNamespace}); err != nil {
			return "", err
		}
	}
	return ctxResource.Context, nil
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
