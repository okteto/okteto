package remote

import (
	"context"
	"fmt"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/spf13/afero"
	"path/filepath"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/build"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/types"
)

// OktetoBuilderInterface runs the build of an image
type oktetoBuilderInterface interface {
	Run(ctx context.Context, buildOptions *types.BuildOptions) error
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
}

// NewBuilderFromScratch creates a new okteto builder
func NewBuilderFromScratch() *OktetoBuilder {
	builder := &build.OktetoBuilder{}
	registry := registry.NewOktetoRegistry(okteto.Config{})
	return &OktetoBuilder{
		Builder:  builder,
		Registry: registry,
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
		return fmt.Errorf("invalid build context: %s", err.Error())
	}
	options.Path = path

	if options.File == "" {
		options.File = filepath.Join(path, "Dockerfile")
	}

	if exists := filesystem.FileExistsAndNotDir(options.File, afero.NewOsFs()); !exists {
		return fmt.Errorf("%s: '%s' is not a regular file", oktetoErrors.InvalidDockerfile, options.File)
	}

	if err := bc.Builder.Run(ctx, options); err != nil {
		analytics.TrackBuild(false)
		return err
	}

	analytics.TrackBuild(true)
	return nil
}

func (bc *OktetoBuilder) IsV1() bool {
	return true
}
