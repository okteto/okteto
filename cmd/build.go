package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/containerd/console"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/util/progress/progressui"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/forward"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"golang.org/x/sync/errgroup"

	"github.com/spf13/cobra"
)

const (
	frontend          = "dockerfile.v0"
	buildKitContainer = "buildkit-0"
	buildKitPort      = 1234
)

//Build build and optionally push a Docker image
func Build() *cobra.Command {
	var file string
	var tag string
	var target string
	var noCache bool

	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build (and optionally push) a Docker image",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debug("starting build command")
			err := RunBuild(args[0], file, tag, target, noCache)
			return err
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("build requires the PATH context argument (e.g. '.' for the current directory)")
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&file, "file", "f", "", "name of the Dockerfile (Default is 'PATH/Dockerfile')")
	cmd.Flags().StringVarP(&tag, "tag", "t", "", "name and optionally a tag in the 'name:tag' format (it is automatically) pushed")
	cmd.Flags().StringVarP(&target, "target", "", "", "set the target build stage to build")
	cmd.Flags().BoolVarP(&noCache, "no-cache", "", false, "do not use cache when building the image")
	return cmd
}

//RunBuild starts the build sequence
func RunBuild(path, file, tag, target string, noCache bool) error {
	ctx := context.Background()

	buildKitHost, err := getBuildKitHost()
	if err != nil {
		return err
	}

	c, err := client.New(ctx, buildKitHost, client.WithFailFast())
	if err != nil {
		return err
	}

	log.Infof("created buildkit client: %+v", c)

	ch := make(chan *client.SolveStatus)
	eg, ctx := errgroup.WithContext(ctx)
	solveOpt, err := getSolveOpt(path, file, tag, target, noCache)
	if err != nil {
		return err
	}

	eg.Go(func() error {
		_, err := c.Solve(ctx, nil, *solveOpt, ch)
		return err
	})

	eg.Go(func() error {
		var c console.Console
		if cn, err := console.ConsoleFromFile(os.Stderr); err == nil {
			c = cn
		}
		// not using shared context to not disrupt display but let it finish reporting errors
		return progressui.DisplaySolveStatus(context.TODO(), "", c, os.Stdout, ch)
	})

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

func getBuildKitHost() (string, error) {
	buildKitHost := os.Getenv("BUILDKIT_HOST")
	if buildKitHost != "" {
		return buildKitHost, nil
	}

	c, restConfig, namespace, err := k8Client.GetLocal()
	if err != nil {
		return "", fmt.Errorf("The variable 'BUILDKIT_HOST' is not defined and current namespace cannot be queried: %s", err)
	}
	ns, err := namespaces.Get(namespace, c)
	if err != nil {
		return "", fmt.Errorf("The variable 'BUILDKIT_HOST' is not defined and current namespace cannot be queried: %s", err)
	}

	if !namespaces.IsOktetoNamespace(ns) {
		return "", fmt.Errorf("The variable 'BUILDKIT_HOST' is not defined")
	}

	ctx := context.Background()
	internalNamespace, err := okteto.GetOkteoInternalNamespace(ctx)
	if err != nil {
		return "", err
	}

	localPort, err := model.GetAvailablePort()
	if err != nil {
		return "", err
	}

	forwarder := forward.NewPortForwardManager(ctx, restConfig, c)
	if err := forwarder.Add(localPort, buildKitPort); err != nil {
		return "", err
	}
	if err := forwarder.Start(buildKitContainer, internalNamespace); err != nil {
		return "", err
	}
	return fmt.Sprintf("tcp://localhost:%d", localPort), nil
}

func getSolveOpt(buildCtx, file, tag, target string, noCache bool) (*client.SolveOpt, error) {
	if file == "" {
		file = filepath.Join(buildCtx, "Dockerfile")
	}
	localDirs := map[string]string{
		"context":    buildCtx,
		"dockerfile": filepath.Dir(file),
	}

	frontendAttrs := map[string]string{
		"filename": filepath.Base(file),
	}
	if target != "" {
		frontendAttrs["target"] = target
	}
	if noCache {
		frontendAttrs["no-cache"] = ""
	}

	// read docker credentials from `.docker/config.json`
	attachable := []session.Attachable{authprovider.NewDockerAuthProvider(os.Stderr)}
	opt := &client.SolveOpt{
		LocalDirs:     localDirs,
		Frontend:      frontend,
		FrontendAttrs: frontendAttrs,
		Session:       attachable,
	}

	if tag != "" {
		opt.Exports = []client.ExportEntry{
			{
				Type: "image",
				Attrs: map[string]string{
					"name": tag,
					"push": "true",
				},
			},
		}
		opt.CacheExports = []client.CacheOptionsEntry{
			{
				Type: "inline",
			},
		}
		opt.CacheImports = []client.CacheOptionsEntry{
			{
				Type:  "registry",
				Attrs: map[string]string{"ref": tag},
			},
		}
	}

	return opt, nil
}
