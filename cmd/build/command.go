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

	buildv1 "github.com/okteto/okteto/cmd/build/v1"
	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	"github.com/okteto/okteto/pkg/cmd/build"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
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
			options.CommandArgs = args
			options.Manifest = manifest

			builder := bc.getBuilder(isBuildV2)
			if err := builder.LoadContext(ctx, options); err != nil {
				return err
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

func (bc *Command) getBuilder(isBuildV2 bool) Builder {
	if isBuildV2 {
		return buildv2.NewBuilder(bc.Builder, bc.Registry)
	}
	return buildv1.NewBuilder(bc.Builder, bc.Registry)
}
