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
	"github.com/okteto/okteto/pkg/analytics"
	buildCmd "github.com/okteto/okteto/pkg/cmd/build"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type outputFormat string

const (
	// TTYFormat is the default format for the output
	TTYFormat outputFormat = "tty"
)

// Command defines the build command
type Command struct {
	GetManifest func(path string, fs afero.Fs) (*model.Manifest, error)

	Builder          buildCmd.OktetoBuilderInterface
	Registry         registryInterface
	analyticsTracker buildTrackerInterface
	insights         buildTrackerInterface
	ioCtrl           *io.Controller
	k8slogger        *io.K8sLogger
}

type buildTrackerInterface interface {
	TrackImageBuild(ctx context.Context, meta *analytics.ImageBuildMetadata)
}

type registryInterface interface {
	GetImageTagWithDigest(imageTag string) (string, error)
	IsOktetoRegistry(image string) bool
	GetImageReference(image string) (registry.OktetoImageReference, error)
	HasGlobalPushAccess() (bool, error)
	IsGlobalRegistry(image string) bool

	GetRegistryAndRepo(image string) (string, string)
	GetRepoNameAndTag(repo string) (string, string)
	GetDevImageFromGlobal(imageWithDigest string) string
	Clone(from, to string) (string, error)
}

// NewBuildCommand creates a struct to run all build methods
func NewBuildCommand(ioCtrl *io.Controller, analyticsTracker, insights buildTrackerInterface, okCtx *okteto.ContextStateless, k8slogger *io.K8sLogger) *Command {
	return &Command{
		GetManifest:      model.GetManifestV2,
		Builder:          buildCmd.NewOktetoBuilder(okCtx, afero.NewOsFs()),
		Registry:         registry.NewOktetoRegistry(buildCmd.GetRegistryConfigFromOktetoConfig(okCtx)),
		ioCtrl:           ioCtrl,
		k8slogger:        k8slogger,
		analyticsTracker: analyticsTracker,
		insights:         insights,
	}
}

const (
	maxV1CommandArgs = 1
	docsURL          = "https://okteto.com/docs/reference/okteto-cli/#build"
)

// Build build and optionally push a Docker image
func Build(ctx context.Context, ioCtrl *io.Controller, at, insights buildTrackerInterface, k8slogger *io.K8sLogger) *cobra.Command {
	options := &types.BuildOptions{}
	cmd := &cobra.Command{
		Use:   "build [service...]",
		Short: "Build and push the images defined in the 'build' section of your okteto manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			options.CommandArgs = args
			// The context must be loaded before reading manifest. Otherwise,
			// secrets will not be resolved when GetManifest is called and
			// the manifest will load empty values.
			oktetoContext, err := getOktetoContext(ctx, options)
			if err != nil {
				return err
			}

			if !okteto.IsOkteto() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}

			ioCtrl.Logger().Info("context loaded")

			bc := NewBuildCommand(ioCtrl, at, insights, oktetoContext, k8slogger)

			builder, err := bc.getBuilder(options, oktetoContext)

			if err != nil {
				return err
			}

			if builder.IsV1() {
				if len(options.CommandArgs) > maxV1CommandArgs {
					return oktetoErrors.UserError{
						E:    fmt.Errorf("when passing a context to 'okteto build', it accepts at most %d arg(s), but received %d", maxV1CommandArgs, len(options.CommandArgs)),
						Hint: fmt.Sprintf("Visit %s for more information.", docsURL),
					}
				}
			}

			analytics.TrackBuildWithManifestVsDockerfile(builder.IsV1())
			return builder.Build(ctx, options)
		},
	}

	cmd.Flags().StringVarP(&options.K8sContext, "context", "c", "", "context where the build command is executed")
	cmd.Flags().StringVarP(&options.File, "file", "f", "", "path to the Okteto manifest file")
	cmd.Flags().StringVarP(&options.Tag, "tag", "t", "", "name and optionally a tag in the 'name:tag' format (it is automatically pushed)")
	cmd.Flags().StringVarP(&options.Target, "target", "", "", "set the target build stage to build")
	cmd.Flags().BoolVarP(&options.NoCache, "no-cache", "", false, "do not use cache when building the image")
	cmd.Flags().StringArrayVar(&options.CacheFrom, "cache-from", nil, "cache source images")
	cmd.Flags().StringArrayVar(&options.ExportCache, "export-cache", nil, "export cache images")
	cmd.Flags().StringVarP(&options.OutputMode, "progress", "", string(TTYFormat), "show plain/tty build output")
	cmd.Flags().StringArrayVar(&options.BuildArgs, "build-arg", nil, "set build-time variables")
	cmd.Flags().StringArrayVar(&options.Secrets, "secret", nil, "secret files exposed to the build. Format: id=mysecret,src=/local/secret")
	cmd.Flags().StringVar(&options.Platform, "platform", "", "set platform if server is multi-platform capable")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "namespace against which the image will be consumed. Default is the one defined at okteto context or okteto manifest")
	return cmd
}

// getBuilder returns the proper builder (V1 or V2) based on the manifest. The following rules are applied:
//   - If the manifest is not found or there is any error getting the manifest, the builder fallsback to V1
//   - If the manifest is found and it is a V2 manifest and the build section has some image, the builder is V2
//   - If the manifest is found and it is a V1 manifest or the build section is empty, the builder fallsback to V1
func (bc *Command) getBuilder(options *types.BuildOptions, okCtx *okteto.ContextStateless) (Builder, error) {
	// the file flag is a Dockerfile
	isDockerfileValid := validateDockerfile(options.File) == nil
	if options.File != "" && isDockerfileValid {
		return buildv1.NewBuilder(bc.Builder, bc.ioCtrl), nil
	}

	var builder Builder
	manifest, err := bc.GetManifest(options.File, afero.NewOsFs())
	if err != nil {
		if options.File != "" && errors.Is(err, oktetoErrors.ErrInvalidManifest) && validateDockerfile(options.File) != nil {
			return nil, err
		}

		bc.ioCtrl.Logger().Infof("manifest located at %s is not v2 compatible: %s", options.File, err)
		bc.ioCtrl.Logger().Info("falling back to building as a v1 manifest")

		builder = buildv1.NewBuilder(bc.Builder, bc.ioCtrl)
	} else {
		if len(manifest.Build) > 0 {
			callbacks := []buildv2.OnBuildFinish{
				bc.analyticsTracker.TrackImageBuild,
				bc.insights.TrackImageBuild,
			}
			builder = buildv2.NewBuilder(bc.Builder, bc.Registry, bc.ioCtrl, okCtx, bc.k8slogger, callbacks)
		} else {
			builder = buildv1.NewBuilder(bc.Builder, bc.ioCtrl)
		}
	}

	options.Manifest = manifest

	return builder, nil
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

func getOktetoContext(ctx context.Context, options *types.BuildOptions) (*okteto.ContextStateless, error) {
	ctxOpts := &contextCMD.Options{
		Context:   options.K8sContext,
		Namespace: options.Namespace,
		Show:      true,
	}

	oktetoContext, err := contextCMD.NewContextCommand().RunStateless(ctx, ctxOpts)
	if err != nil {
		return nil, err
	}

	if oktetoContext.IsOktetoCluster() && ctxOpts.Namespace != "" {
		ocfg := defaultOktetoClientCfg(oktetoContext)
		c, err := okteto.NewOktetoClientStateless(ocfg)
		if err != nil {
			return nil, err
		}

		create, err := utils.ShouldCreateNamespaceStateless(ctx, ctxOpts.Namespace, c)
		if err != nil {
			return nil, err
		}
		if create {
			if err := namespace.NewCommandStateless(c).Create(ctx, &namespace.CreateOptions{Namespace: ctxOpts.Namespace}); err != nil {
				return nil, err
			}
		}
	}

	return oktetoContext, err
}

type oktetoClientCfgContext interface {
	ExistsContext() bool
	GetCurrentName() string
	GetCurrentToken() string
	GetCurrentCertStr() string
}

func defaultOktetoClientCfg(octx oktetoClientCfgContext) *okteto.ClientCfg {
	if !octx.ExistsContext() {
		return &okteto.ClientCfg{}
	}

	return &okteto.ClientCfg{
		CtxName: octx.GetCurrentName(),
		Token:   octx.GetCurrentToken(),
		Cert:    octx.GetCurrentCertStr(),
	}

}
