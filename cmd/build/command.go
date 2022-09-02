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

package build

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/moby/buildkit/frontend/dockerfile/instructions"
	"github.com/moby/buildkit/frontend/dockerfile/parser"
	buildv1 "github.com/okteto/okteto/cmd/build/v1"
	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/namespace"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/cmd/build"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/cobra"
)

// Command defines the build command
type Command struct {
	GetManifest func(path string) (*model.Manifest, error)

	Builder  build.OktetoBuilderInterface
	Registry build.OktetoRegistryInterface
}

// NewBuildCommand creates a struct to run all build methods
func NewBuildCommand() *Command {
	return &Command{
		GetManifest: model.GetManifestV2,
		Builder:     &build.OktetoBuilder{},
		Registry:    registry.NewOktetoRegistry(),
	}
}

// Build build and optionally push a Docker image
func Build(ctx context.Context) *cobra.Command {

	options := &types.BuildOptions{}
	cmd := &cobra.Command{
		Use:   "build [service...]",
		Short: "Build and push the images defined in the 'build' section of your okteto manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			options.CommandArgs = args
			bc := NewBuildCommand()

			// The context must be loaded before the read manifest. Otherwise,
			// secrets will not be resolved when GetManifest is called and
			// the manifest will load empty values.
			if err := bc.loadContext(ctx, options); err != nil {
				return err
			}

			builder, err := bc.getBuilder(options)
			if err != nil {
				return err
			}

			switch builder.(type) {
			case *buildv1.OktetoBuilder:
				maxV1Args := 1
				docsURL := "https://okteto.com/docs/reference/cli/#build"
				if len(options.CommandArgs) > maxV1Args {
					return oktetoErrors.UserError{
						E:    fmt.Errorf("when passing a context to 'okteto build', it accepts at most %d arg(s), but received %d", maxV1Args, len(options.CommandArgs)),
						Hint: fmt.Sprintf("Visit %s for more information.", docsURL),
					}
				}
			}

			return builder.Build(ctx, options)
		},
	}

	cmd.Flags().StringVarP(&options.K8sContext, "context", "c", "", "context where the build command is executed")
	cmd.Flags().StringVarP(&options.File, "file", "f", "", "path to the Okteto Manifest (default is 'okteto.yml')")
	cmd.Flags().StringVarP(&options.Tag, "tag", "t", "", "name and optionally a tag in the 'name:tag' format (it is automatically pushed)")
	cmd.Flags().StringVarP(&options.Target, "target", "", "", "set the target build stage to build")
	cmd.Flags().BoolVarP(&options.NoCache, "no-cache", "", false, "do not use cache when building the image")
	cmd.Flags().StringArrayVar(&options.CacheFrom, "cache-from", nil, "cache source images")
	cmd.Flags().StringVarP(&options.ExportCache, "export-cache", "", "", "export cache image")
	cmd.Flags().StringVarP(&options.OutputMode, "progress", "", oktetoLog.TTYFormat, "show plain/tty build output")
	cmd.Flags().StringArrayVar(&options.BuildArgs, "build-arg", nil, "set build-time variables")
	cmd.Flags().StringArrayVar(&options.Secrets, "secret", nil, "secret files exposed to the build. Format: id=mysecret,src=/local/secret")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "namespace against which the image will be consumed. Default is the one defined at okteto context or okteto manifest")
	cmd.Flags().BoolVarP(&options.BuildToGlobal, "global", "", false, "push the image to the global registry")
	return cmd
}

func (bc *Command) getBuilder(options *types.BuildOptions) (Builder, error) {
	var builder Builder

	manifest, err := bc.GetManifest(options.File)
	if err != nil {
		if options.File != "" && errors.Is(err, oktetoErrors.ErrInvalidManifest) && validateDockerfile(options.File) != nil {
			return nil, err
		}

		oktetoLog.Infof("The manifest %s is not v2 compatible, falling back to building as a v1 manifest: %v", options.File, err)
		builder = buildv1.NewBuilder(bc.Builder, bc.Registry)
	} else {
		if isBuildV2(manifest) {
			builder = buildv2.NewBuilder(bc.Builder, bc.Registry)
		} else {
			builder = buildv1.NewBuilder(bc.Builder, bc.Registry)
		}
	}

	options.Manifest = manifest

	return builder, nil
}

func isBuildV2(m *model.Manifest) bool {
	return m.IsV2 && len(m.Build) != 0
}

func validateDockerfile(file string) error {
	dat, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	parsedDockerfile, err := parser.Parse(bytes.NewBuffer(dat))
	if err != nil {
		return err
	}

	_, _, err = instructions.Parse(parsedDockerfile.AST)
	return err
}

func (*Command) loadContext(ctx context.Context, options *types.BuildOptions) error {
	ctxOpts := &contextCMD.ContextOptions{}

	// before calling the context command, there is need to retrieve the context and
	// namespace through the given manifest. If the manifest is a Dockerfile, this
	// information cannot be extracted so call to GetContextResource is skkiped.
	if err := validateDockerfile(options.File); err != nil {
		ctxResource, err := model.GetContextResource(options.File)
		if err != nil {
			return err
		}

		if options.Namespace != "" {
			ctxOpts.Namespace = ctxResource.Namespace
		}

		if options.Namespace != "" {
			ctxOpts.Context = ctxResource.Context
		}
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
