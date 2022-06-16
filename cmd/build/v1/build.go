// Copyright 2022 The Okteto Authors
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

package v1

import (
	"context"
	"fmt"
	"path/filepath"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/namespace"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/errors"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/types"
)

// OktetoBuilderInterface runs the build of an image
type OktetoBuilderInterface interface {
	Run(ctx context.Context, buildOptions *types.BuildOptions) error
}

// OktetoBuilder builds the images
type OktetoBuilder struct {
	Builder  OktetoBuilderInterface
	Registry build.OktetoRegistryInterface
}

// NewBuilder creates a new okteto builder
func NewBuilder(builder OktetoBuilderInterface, registry build.OktetoRegistryInterface) *OktetoBuilder {
	return &OktetoBuilder{
		Builder:  builder,
		Registry: registry,
	}
}

// NewBuilderFromScratch creates a new okteto builder
func NewBuilderFromScratch() *OktetoBuilder {
	builder := &build.OktetoBuilder{}
	registry := registry.NewOktetoRegistry()
	return &OktetoBuilder{
		Builder:  builder,
		Registry: registry,
	}
}

// LoadContext Loads the okteto context based on a build v1
func (bc *OktetoBuilder) LoadContext(ctx context.Context, options *types.BuildOptions) error {
	ctxOpts := &contextCMD.ContextOptions{}
	maxV1Args := 1
	docsURL := "https://okteto.com/docs/reference/cli/#build"
	if len(options.CommandArgs) > maxV1Args {
		return oktetoErrors.UserError{
			E:    fmt.Errorf("when passing a context to 'okteto build', it accepts at most %d arg(s), but received %d", maxV1Args, len(options.CommandArgs)),
			Hint: fmt.Sprintf("Visit %s for more information.", docsURL),
		}
	}

	if options.Namespace != "" {
		ctxOpts.Namespace = options.Namespace
	}

	if options.K8sContext != "" {
		ctxOpts.Context = options.K8sContext
	}

	if okteto.IsOkteto() && ctxOpts.Namespace != "" {
		create, err := utils.ShouldCreateNamespace(ctx, ctxOpts.Namespace)
		if err != nil {
			return err
		}
		if create {
			nsCmd, err := namespace.NewCommand()
			if err != nil {
				return err
			}
			if err := nsCmd.Create(ctx, &namespace.CreateOptions{Namespace: ctxOpts.Namespace}); err != nil {
				return err
			}
		}
	}

	return contextCMD.NewContextCommand().Run(ctx, ctxOpts)
}

// Build builds the images defined by a Dockerfile
func (bc *OktetoBuilder) Build(ctx context.Context, options *types.BuildOptions) error {
	path := "."
	if options.Path != "" {
		path = options.Path
	}
	if len(options.CommandArgs) == 1 {
		path = options.CommandArgs[0]
	}

	if err := utils.CheckIfDirectory(path); err != nil {
		return fmt.Errorf("invalid build context: %s", err.Error())
	}
	options.Path = path

	if options.File == "" {
		options.File = filepath.Join(path, "Dockerfile")
	}

	if err := utils.CheckIfRegularFile(options.File); err != nil {
		return fmt.Errorf("%s: %s", errors.InvalidDockerfile, err.Error())
	}

	buildMsg := "Building the image"
	if options.Tag != "" {
		buildMsg = fmt.Sprintf("Building the image '%s'", options.Tag)
	}
	if okteto.Context().Builder == "" {
		oktetoLog.Information("%s using your local docker daemon", buildMsg)
	} else {
		oktetoLog.Information("%s in %s...", buildMsg, okteto.Context().Builder)
	}

	if err := bc.Builder.Run(ctx, options); err != nil {
		analytics.TrackBuild(okteto.Context().Builder, false)
		return err
	}

	if options.Tag == "" {
		oktetoLog.Success("Build succeeded")
		oktetoLog.Information("Your image won't be pushed. To push your image specify the flag '-t'.")
	} else {
		oktetoLog.Success(fmt.Sprintf("Image '%s' successfully pushed", options.Tag))
	}

	analytics.TrackBuild(okteto.Context().Builder, true)
	return nil
}
