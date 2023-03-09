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

package v1

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/build"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/repository"
	"github.com/okteto/okteto/pkg/types"
)

// OktetoBuilderInterface runs the build of an image
type OktetoBuilderInterface interface {
	Run(ctx context.Context, buildOptions *types.BuildOptions) error
}

type oktetoRegistryInterface interface {
	GetImageTagWithDigest(imageTag string) (string, error)
	HasGlobalPushAcces() (bool, error)
}

type oktetoBuilderConfig struct {
	hasGlobalAccess bool
	isCleanProject  bool
}

// OktetoBuilder builds the images
type OktetoBuilder struct {
	Builder  OktetoBuilderInterface
	Registry oktetoRegistryInterface

	config oktetoBuilderConfig

	gitRepository        repository.Repository
	workingDirectoryCtrl filesystem.WorkingDirectoryInterface
}

// NewBuilder creates a new okteto builder
func NewBuilder(builder OktetoBuilderInterface, registry oktetoRegistryInterface) *OktetoBuilder {
	wdCtrl := filesystem.NewOsWorkingDirectoryCtrl()
	wd, err := wdCtrl.Get()
	if err != nil {
		oktetoLog.Infof("could not get working dir: %w", err)
	}
	gitRepo := repository.NewRepository(wd)
	return &OktetoBuilder{
		Builder:              builder,
		Registry:             registry,
		gitRepository:        gitRepo,
		workingDirectoryCtrl: wdCtrl,

		config: getConfig(registry, gitRepo),
	}
}

// NewBuilderFromScratch creates a new okteto builder
func NewBuilderFromScratch() *OktetoBuilder {
	builder := &build.OktetoBuilder{}
	registry := registry.NewOktetoRegistry(okteto.Config{})
	return NewBuilder(builder, registry)
}

func getConfig(registry oktetoRegistryInterface, gitRepo repository.Repository) oktetoBuilderConfig {
	hasAccess, err := registry.HasGlobalPushAcces()
	if err != nil {
		oktetoLog.Infof("error trying to access globalPushAccess: %w", err)
	}

	isClean, err := gitRepo.IsClean()
	if err != nil {
		oktetoLog.Infof("error trying to get directory: %w", err)
	}
	return oktetoBuilderConfig{
		hasGlobalAccess: hasAccess,
		isCleanProject:  isClean,
	}
}

// HasGlobalAccess checks if the user has access to global registry
func (ob *OktetoBuilder) HasGlobalAccess() bool {
	return ob.config.hasGlobalAccess
}

// IsCleanProject checks if the repository is clean(no changes over the last commit)
func (ob *OktetoBuilder) IsCleanProject() bool {
	return ob.config.isCleanProject
}

// IsV1 returns true since it is a builder v1
func (*OktetoBuilder) IsV1() bool {
	return true
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
		return fmt.Errorf("%s: %s", oktetoErrors.InvalidDockerfile, err.Error())
	}

	buildMsg := fmt.Sprintf("Building '%s'", options.File)
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
