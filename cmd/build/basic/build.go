package basic

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
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
)

// BuildRunner runs the build of an image
type BuildRunner interface {
	Run(ctx context.Context, buildOptions *types.BuildOptions, ioCtrl *io.Controller) error
}

// Builder It provides basic functionality to build images.
// This might be used as a base for more complex builders (e.g. v1, v2)
type Builder struct {
	BuildRunner BuildRunner
	IoCtrl      *io.Controller
}

// NewBuilderFromScratch creates a new okteto builder
func NewBuilderFromScratch(ioCtrl *io.Controller) *Builder {
	builder := &buildCmd.OktetoBuilder{
		OktetoContext: &okteto.ContextStateless{
			Store: okteto.GetContextStore(),
		},
		Fs: afero.NewOsFs(),
	}

	return &Builder{
		BuildRunner: builder,
		IoCtrl:      ioCtrl,
	}
}

// Build builds the image defined by the BuildOptions used the BuildRunner passed as dependency
// of the builder
func (ob *Builder) Build(ctx context.Context, options *types.BuildOptions) error {
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

	if err := ob.BuildRunner.Run(ctx, options, ob.IoCtrl); err != nil {
		analytics.TrackBuild(false)
		return err
	}

	analytics.TrackBuild(true)
	return nil
}

func (bc *Builder) IsV1() bool {
	return false
}
