// Copyright 2023 The Okteto Authors
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

package deploy

import (
	"context"
	"math/rand"
	"os"
	"text/template"

	buildV1 "github.com/okteto/okteto/cmd/build/v1"
	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/constants"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
)

const (
	oktetoCLIImage     = "gcr.io/development-300207/okteto:remote-deploy"
	dockerfileTemplate = `
FROM {{ .OktetoCLIImage }} as okteto-cli

FROM alpine as certs
RUN apk update && apk add ca-certificates

FROM {{ .UserDeployImage }} as deploy

ENV PATH="${PATH}:/app/bin"
COPY --from=certs /etc/ssl/certs /etc/ssl/certs
COPY --from=okteto-cli /usr/local/bin/okteto /usr/local/bin/okteto

{{range $key, $val := .OktetoBuildEnvVars }}
ENV {{$key}} {{$val}}
{{end}}
ENV {{ .NamespaceEnvVar }} {{ .NamespaceValue }}
ENV {{ .ContextEnvVar }} {{ .ContextValue }}
ENV {{ .TokenEnvVar }} {{ .TokenValue }}
ENV {{ .RemoteDeployEnvVar }} true

COPY . /okteto/app
WORKDIR /okteto/app

ENV OKTETO_INVALIDATE_CACHE {{ .RandomInt }}
RUN okteto deploy --log-output=json
`
)

type dockerfileTemplateProperties struct {
	OktetoCLIImage     string
	UserDeployImage    string
	OktetoBuildEnvVars map[string]string
	ContextEnvVar      string
	ContextValue       string
	NamespaceEnvVar    string
	NamespaceValue     string
	TokenEnvVar        string
	TokenValue         string
	RemoteDeployEnvVar string
	RandomInt          int
}

type remoteDeployCommand struct {
	builder *buildv2.OktetoBuilder
}

func newRemoteDeployer(builder *buildv2.OktetoBuilder) *remoteDeployCommand {
	return &remoteDeployCommand{
		builder: builder,
	}
}

func (rd *remoteDeployCommand) deploy(ctx context.Context, deployOptions *Options) error {

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	c, _, err := okteto.NewK8sClientProvider().Provide(okteto.Context().Cfg)
	if err != nil {
		return err
	}

	setDeployOptionsValuesFromManifest(ctx, deployOptions, cwd, c)

	tmpl, err := template.New("dockerfile").Parse(dockerfileTemplate)
	if err != nil {
		return err
	}
	dockerfileSyntax := dockerfileTemplateProperties{
		OktetoCLIImage:     oktetoCLIImage,
		UserDeployImage:    deployOptions.Manifest.Deploy.Image,
		OktetoBuildEnvVars: rd.builder.GetBuildEnvVars(),
		ContextEnvVar:      model.OktetoContextEnvVar,
		ContextValue:       okteto.Context().Name,
		NamespaceEnvVar:    model.OktetoNamespaceEnvVar,
		NamespaceValue:     okteto.Context().Namespace,
		TokenEnvVar:        model.OktetoTokenEnvVar,
		TokenValue:         okteto.Context().Token,
		RemoteDeployEnvVar: constants.OKtetoDeployRemote,
		RandomInt:          rand.Intn(1000),
	}

	fs := afero.NewOsFs()

	dockerfile, err := afero.TempFile(fs, "", "Dockerfile.okteto.deploy")
	if err != nil {
		return err
	}

	defer func() {
		if err := fs.Remove(dockerfile.Name()); err != nil {
			oktetoLog.Infof("error removing dockerfile: %w", err)
		}
	}()
	if err := tmpl.Execute(dockerfile, dockerfileSyntax); err != nil {
		return err
	}

	buildInfo := &model.BuildInfo{
		Dockerfile: dockerfile.Name(),
	}

	buildOptions := build.OptsFromBuildInfo("", "", buildInfo, &types.BuildOptions{Path: cwd, OutputMode: "deploy"})
	buildOptions.Tag = ""
	if err := buildV1.NewBuilderFromScratch().Build(ctx, buildOptions); err != nil {
		return err
	}

	return nil
}

func (rd *remoteDeployCommand) cleanUp(ctx context.Context, err error) {
	//TODO
	return
}
