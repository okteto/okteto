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
	"context"
	"fmt"

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
			bc := NewBuildCommand()

			manifest, isBuildV2 := bc.getManifestAndBuildVersion(options)
			options.Args = args
			if err := loadContext(ctx, manifest, isBuildV2, options); err != nil {
				return err
			}

			if isBuildV2 {
				return bc.BuildV2(ctx, manifest, options)
			}

			return bc.BuildV1(ctx, options)
		},
	}

	cmd.Flags().StringVarP(&options.File, "file", "f", "", "path to the Okteto Manifest (default is 'okteto.yml')")
	cmd.Flags().StringVarP(&options.Tag, "tag", "t", "", "name and optionally a tag in the 'name:tag' format (it is automatically pushed)")
	cmd.Flags().StringVarP(&options.Target, "target", "", "", "set the target build stage to build")
	cmd.Flags().BoolVarP(&options.NoCache, "no-cache", "", false, "do not use cache when building the image")
	cmd.Flags().StringArrayVar(&options.CacheFrom, "cache-from", nil, "cache source images")
	cmd.Flags().StringVarP(&options.OutputMode, "progress", "", oktetoLog.TTYFormat, "show plain/tty build output")
	cmd.Flags().StringArrayVar(&options.BuildArgs, "build-arg", nil, "set build-time variables")
	cmd.Flags().StringArrayVar(&options.Secrets, "secret", nil, "secret files exposed to the build. Format: id=mysecret,src=/local/secret")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "", "", "namespace against which the image will be consumed. Default is the one defined at okteto context or okteto manifest")
	cmd.Flags().BoolVarP(&options.BuildToGlobal, "global", "", false, "push the image to the global registry")
	return cmd
}

func (bc *Command) getManifestAndBuildVersion(options *types.BuildOptions) (*model.Manifest, bool) {
	manifest, errManifest := bc.GetManifest(options.File)
	if errManifest != nil {
		oktetoLog.Debug("error getting manifest v2 from file %s: %v. Fallback to build v1", options.File, errManifest)
	}

	isBuildV2 := errManifest == nil &&
		manifest.IsV2 &&
		len(manifest.Build) != 0
	return manifest, isBuildV2
}

func loadContext(ctx context.Context, manifest *model.Manifest, isBuildV2 bool, options *types.BuildOptions) error {
	ctxOpts := &contextCMD.ContextOptions{}
	if isBuildV2 {
		if manifest.Context != "" {
			ctxOpts.Context = manifest.Context
			if err := contextCMD.NewContextCommand().Run(ctx, ctxOpts); err != nil {
				return err
			}
		}

		if options.Namespace == "" && manifest.Namespace != "" {
			ctxOpts.Namespace = manifest.Namespace
		}
	} else {
		maxV1Args := 1
		docsURL := "https://okteto.com/docs/reference/cli/#build"
		if len(options.Args) > maxV1Args {
			return oktetoErrors.UserError{
				E:    fmt.Errorf("when passing a context to 'okteto build', it accepts at most %d arg(s), but received %d", maxV1Args, len(options.Args)),
				Hint: fmt.Sprintf("Visit %s for more information.", docsURL),
			}
		}

		if options.Namespace != "" {
			ctxOpts.Namespace = options.Namespace
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

	if err := contextCMD.NewContextCommand().Run(ctx, ctxOpts); err != nil {
		return err
	}
	return nil
}
