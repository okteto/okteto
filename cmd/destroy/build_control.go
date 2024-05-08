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

package destroy

import (
	"context"
	"fmt"
	"strings"

	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/types"
)

type buildCtrl struct {
	builder builderInterface
	name    string
}

func newBuildCtrl(name string, analyticsTracker, insights buildTrackerInterface, ioCtrl *io.Controller) buildCtrl {
	onBuildFinish := []buildv2.OnBuildFinish{
		analyticsTracker.TrackImageBuild,
		insights.TrackImageBuild,
	}
	return buildCtrl{
		builder: buildv2.NewBuilderFromScratch(ioCtrl, onBuildFinish),
		name:    name,
	}
}

type builderInterface interface {
	GetServicesToBuildForImage(ctx context.Context, manifest *model.Manifest, findImg model.ImageFromManifest) ([]string, error)
	Build(ctx context.Context, options *types.BuildOptions) error
}

func (bc buildCtrl) buildImageIfNecessary(ctx context.Context, manifest *model.Manifest) error {
	oktetoLog.Debug("checking if destroy.image is already built")
	svcsToBuild, err := bc.builder.GetServicesToBuildForImage(ctx, manifest, func(manifest *model.Manifest) string {
		return manifest.Destroy.Image
	})
	if err != nil {
		return err
	}
	if len(svcsToBuild) < 1 {
		return nil
	}
	buildOptions := &types.BuildOptions{
		EnableStages: true,
		Manifest:     manifest,
		CommandArgs:  svcsToBuild,
	}
	oktetoLog.Infof("rebuilding %s services image", strings.Join(svcsToBuild, "|"))
	if errBuild := bc.builder.Build(ctx, buildOptions); errBuild != nil {
		return fmt.Errorf("error building images: %w", errBuild)
	}
	return nil
}
