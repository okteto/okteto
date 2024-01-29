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

package remote

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	buildCmd "github.com/okteto/okteto/pkg/cmd/build"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
)

// OktetoBuilderInterface runs the build of an image
type oktetoBuilderInterface interface {
	Run(ctx context.Context, buildOptions *types.BuildOptions, ioCtrl *io.Controller) error
}

type OktetoRegistryInterface interface {
	GetImageTagWithDigest(imageTag string) (string, error)
	IsOktetoRegistry(image string) bool
	HasGlobalPushAccess() (bool, error)
	IsGlobalRegistry(image string) bool

	GetRegistryAndRepo(image string) (string, string)
	GetRepoNameAndTag(repo string) (string, string)
}

// OktetoBuilder builds the images
type OktetoBuilder struct {
	Builder  oktetoBuilderInterface
	Registry OktetoRegistryInterface
	ioCtrl   *io.Controller
}

// NewBuilderFromScratch creates a new okteto builder
func NewBuilderFromScratch(ioCtrl *io.Controller) *OktetoBuilder {
	builder := &buildCmd.OktetoBuilder{
		OktetoContext: &okteto.ContextStateless{
			Store: okteto.GetContextStore(),
		},
		Fs: afero.NewOsFs(),
	}
	registry := registry.NewOktetoRegistry(okteto.Config{})
	return &OktetoBuilder{
		Builder:  builder,
		Registry: registry,
		ioCtrl:   ioCtrl,
	}
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
		return fmt.Errorf("invalid build context: %w", err)
	}
	options.Path = path

	if options.File == "" {
		options.File = filepath.Join(path, "Dockerfile")
	}

	if exists := filesystem.FileExistsAndNotDir(options.File, afero.NewOsFs()); !exists {
		return fmt.Errorf("%s: '%s' is not a regular file", oktetoErrors.InvalidDockerfile, options.File)
	}

	if err := bc.Builder.Run(ctx, options, bc.ioCtrl); err != nil {
		analytics.TrackBuild(false)
		return err
	}

	analytics.TrackBuild(true)
	return nil
}

func (bc *OktetoBuilder) IsV1() bool {
	return true
}
