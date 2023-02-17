package destroy

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	remoteBuild "github.com/okteto/okteto/cmd/build/remote"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"

	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/constants"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
)

const (
	dockerfileTemplate = `
FROM {{ .OktetoCLIImage }} as okteto-cli

FROM alpine as certs
RUN apk update && apk add ca-certificates

FROM {{ .UserDestroyImage }} as deploy

ENV PATH="${PATH}:/okteto/bin"
COPY --from=certs /etc/ssl/certs /etc/ssl/certs
COPY --from=okteto-cli /usr/local/bin/* /okteto/bin/

{{range $key, $val := .OktetoBuildEnvVars }}
ENV {{$key}} {{$val}}
{{end}}
ENV {{ .NamespaceEnvVar }} {{ .NamespaceValue }}
ENV {{ .ContextEnvVar }} {{ .ContextValue }}
ENV {{ .TokenEnvVar }} {{ .TokenValue }}
ENV {{ .RemoteDeployEnvVar }} true

COPY . /okteto/src
WORKDIR /okteto/src

ENV OKTETO_INVALIDATE_CACHE {{ .RandomInt }}
RUN okteto destroy {{ .DestroyFlags }}
`
)

type dockerfileTemplateProperties struct {
	OktetoCLIImage     string
	UserDestroyImage   string
	OktetoBuildEnvVars map[string]string
	ContextEnvVar      string
	ContextValue       string
	NamespaceEnvVar    string
	NamespaceValue     string
	TokenEnvVar        string
	TokenValue         string
	RemoteDeployEnvVar string
	DeployFlags        string
	RandomInt          int
	DestroyFlags       string
}

type remoteDestroyCommand struct {
	manifest *model.Manifest
	fs       afero.Fs
}

func newRemoteDestroyer(manifest *model.Manifest) *remoteDestroyCommand {
	return &remoteDestroyCommand{
		manifest: manifest,
		fs:       afero.NewOsFs(),
	}
}

func (rd *remoteDestroyCommand) destroy(ctx context.Context, opts *Options) error {

	if rd.manifest.Destroy.Image == "" {
		rd.manifest.Destroy.Image = constants.OktetoPipelineRunnerImage
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	tmpl, err := template.New("dockerfile").Parse(dockerfileTemplate)
	if err != nil {
		return err
	}

	dockerfileSyntax := dockerfileTemplateProperties{
		OktetoCLIImage:   constants.OktetoCLIImageForRemote,
		UserDestroyImage: rd.manifest.Destroy.Image,
		//OktetoBuildEnvVars: rd.builder.GetBuildEnvVars(),
		ContextEnvVar:      model.OktetoContextEnvVar,
		ContextValue:       okteto.Context().Name,
		NamespaceEnvVar:    model.OktetoNamespaceEnvVar,
		NamespaceValue:     okteto.Context().Namespace,
		TokenEnvVar:        model.OktetoTokenEnvVar,
		TokenValue:         okteto.Context().Token,
		RemoteDeployEnvVar: constants.OKtetoDeployRemote,
		RandomInt:          rand.Intn(1000),
		DestroyFlags:       strings.Join(getDestroyFlags(opts), " "),
	}

	tmpDir, err := afero.TempDir(rd.fs, "", "")
	if err != nil {
		return err
	}

	dockerfile, err := rd.fs.Create(filepath.Join(tmpDir, "deploy"))
	if err != nil {
		return err
	}

	err = rd.createDockerignoreIfNeeded(cwd, tmpDir)
	if err != nil {
		return err
	}

	defer func() {
		if err := rd.fs.Remove(dockerfile.Name()); err != nil {
			oktetoLog.Infof("error removing dockerfile: %w", err)
		}
	}()
	if err := tmpl.Execute(dockerfile, dockerfileSyntax); err != nil {
		return err
	}

	buildInfo := &model.BuildInfo{
		Dockerfile: dockerfile.Name(),
	}

	// undo modification of CWD for Build command
	os.Chdir(cwd)

	buildOptions := build.OptsFromBuildInfo("", "", buildInfo, &types.BuildOptions{Path: cwd, OutputMode: "deploy"})
	buildOptions.Tag = ""

	// we need to call Build() method using a remote builder. This Builder will have
	// the same behavior as the V1 builder but with a different output taking into
	// account that we must not confuse the user with build messages since this logic is
	// executed in the deploy command.
	if err := remoteBuild.NewBuilderFromScratch().Build(ctx, buildOptions); err != nil {
		fmt.Println(err)
		return oktetoErrors.UserError{
			E: fmt.Errorf("Error during development environment deployment."),
		}
	}
	oktetoLog.SetStage("done")
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "EOF")

	return nil
}

func (rd *remoteDestroyCommand) createDockerignoreIfNeeded(cwd, tmpDir string) error {
	dockerignoreFilePath := fmt.Sprintf("%s/%s", cwd, ".oktetodeployignore")
	if _, err := rd.fs.Stat(dockerignoreFilePath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	} else {
		dockerignoreContent, err := afero.ReadFile(rd.fs, dockerignoreFilePath)
		if err != nil {
			return err
		}

		err = afero.WriteFile(rd.fs, fmt.Sprintf("%s/%s", tmpDir, ".dockerignore"), dockerignoreContent, 0600)
		if err != nil {
			return err
		}
	}

	return nil
}

func getDestroyFlags(opts *Options) []string {
	var deployFlags []string

	if opts.Name != "" {
		deployFlags = append(deployFlags, fmt.Sprintf("--name %s", opts.Name))
	}

	if opts.Namespace != "" {
		deployFlags = append(deployFlags, fmt.Sprintf("--namespace %s", opts.Namespace))
	}

	if opts.ManifestPathFlag != "" {
		deployFlags = append(deployFlags, fmt.Sprintf("--file %s", opts.ManifestPathFlag))
	}

	if opts.DestroyVolumes {
		deployFlags = append(deployFlags, "--volumes")
	}

	if opts.ForceDestroy {
		deployFlags = append(deployFlags, "--force-destroy")
	}

	if opts.DestroyAll {
		deployFlags = append(deployFlags, "--all")
	}

	return deployFlags
}
