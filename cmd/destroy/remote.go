package destroy

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	builder "github.com/okteto/okteto/cmd/build"
	remoteBuild "github.com/okteto/okteto/cmd/build/remote"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"

	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/constants"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
)

const (
	templateName           = "destroy-dockerfile"
	dockerfileTemporalNane = "deploy"
	oktetoDockerignoreName = ".oktetodeployignore"
	dockerfileTemplate     = `
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
ENV {{ .ActionNameEnvVar }} {{ .ActionNameValue }}

COPY . /okteto/src
WORKDIR /okteto/src

ENV OKTETO_INVALIDATE_CACHE {{ .RandomInt }}
RUN okteto destroy --log-output=json {{ .DestroyFlags }}
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
	ActionNameEnvVar   string
	ActionNameValue    string
	RemoteDeployEnvVar string
	DeployFlags        string
	RandomInt          int
	DestroyFlags       string
}

type remoteDestroyCommand struct {
	builder              builder.Builder
	destroyImage         string
	fs                   afero.Fs
	workingDirectoryCtrl filesystem.WorkingDirectoryInterface
	temporalCtrl         filesystem.TemporalDirectoryInterface
}

func newRemoteDestroyer(destroyImage string) *remoteDestroyCommand {
	fs := afero.NewOsFs()
	return &remoteDestroyCommand{
		builder:              remoteBuild.NewBuilderFromScratch(),
		destroyImage:         destroyImage,
		fs:                   fs,
		workingDirectoryCtrl: filesystem.NewOsWorkingDirectoryCtrl(),
		temporalCtrl:         filesystem.NewTemporalDirectoryCtrl(fs),
	}
}

func (rd *remoteDestroyCommand) destroy(ctx context.Context, opts *Options) error {

	if rd.destroyImage == "" {
		rd.destroyImage = constants.OktetoPipelineRunnerImage
	}

	cwd, err := rd.workingDirectoryCtrl.Get()
	if err != nil {
		return err
	}

	tmpDir, err := rd.temporalCtrl.Create()
	if err != nil {
		return err
	}

	dockerfile, err := rd.createDockerfile(tmpDir, opts)
	if err != nil {
		return err
	}

	defer func() {
		if err := rd.fs.Remove(dockerfile); err != nil {
			oktetoLog.Infof("error removing dockerfile: %w", err)
		}
	}()

	buildInfo := &model.BuildInfo{
		Dockerfile: dockerfile,
	}

	// undo modification of CWD for Build command
	if err := rd.workingDirectoryCtrl.Change(cwd); err != nil {
		return err
	}

	buildOptions := build.OptsFromBuildInfo("", "", buildInfo, &types.BuildOptions{Path: cwd, OutputMode: "deploy"})
	buildOptions.Tag = ""

	// we need to call Build() method using a remote builder. This Builder will have
	// the same behavior as the V1 builder but with a different output taking into
	// account that we must not confuse the user with build messages since this logic is
	// executed in the deploy command.
	if err := rd.builder.Build(ctx, buildOptions); err != nil {
		return oktetoErrors.UserError{
			E: fmt.Errorf("error during development environment deployment"),
		}
	}
	oktetoLog.SetStage("done")
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "EOF")

	return nil
}

func (rd *remoteDestroyCommand) createDockerfile(tempDir string, opts *Options) (string, error) {
	cwd, err := rd.workingDirectoryCtrl.Get()
	if err != nil {
		return "", err
	}

	randomNumber, err := rand.Int(rand.Reader, big.NewInt(1000))
	if err != nil {
		return "", err
	}

	tmpl := template.Must(template.New(templateName).Parse(dockerfileTemplate))
	dockerfileSyntax := dockerfileTemplateProperties{
		OktetoCLIImage:     getOktetoCLIVersion(config.VersionString),
		UserDestroyImage:   rd.destroyImage,
		ContextEnvVar:      model.OktetoContextEnvVar,
		ContextValue:       okteto.Context().Name,
		NamespaceEnvVar:    model.OktetoNamespaceEnvVar,
		NamespaceValue:     okteto.Context().Namespace,
		TokenEnvVar:        model.OktetoTokenEnvVar,
		TokenValue:         okteto.Context().Token,
		ActionNameEnvVar:   model.OktetoActionNameEnvVar,
		ActionNameValue:    os.Getenv(model.OktetoActionNameEnvVar),
		RemoteDeployEnvVar: constants.OKtetoDeployRemote,
		RandomInt:          int(randomNumber.Int64()),
		DestroyFlags:       strings.Join(getDestroyFlags(opts), " "),
	}

	dockerfile, err := rd.fs.Create(filepath.Join(tempDir, "deploy"))
	if err != nil {
		return "", err
	}

	err = rd.createDockerignoreIfNeeded(cwd, tempDir)
	if err != nil {
		return "", err
	}

	if err := tmpl.Execute(dockerfile, dockerfileSyntax); err != nil {
		return "", err
	}
	return dockerfile.Name(), nil

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

	return deployFlags
}

func getOktetoCLIVersion(versionString string) string {
	var version string
	if match, _ := regexp.MatchString(`\d+\.\d+\.\d+`, versionString); match {
		version = fmt.Sprintf(constants.OktetoCLIImageForRemoteTemplate, versionString)
	} else {
		remoteOktetoImage := os.Getenv(constants.OKtetoDeployRemoteImage)
		if remoteOktetoImage != "" {
			version = remoteOktetoImage
		} else {
			version = fmt.Sprintf(constants.OktetoCLIImageForRemoteTemplate, "latest")
		}
	}

	return version
}
