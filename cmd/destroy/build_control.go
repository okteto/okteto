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
	"regexp"
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

func newBuildCtrl(name string, analyticsTracker analyticsTrackerInterface, ioCtrl *io.Controller) buildCtrl {
	return buildCtrl{
		builder: buildv2.NewBuilderFromScratch(analyticsTracker, ioCtrl),
		name:    name,
	}
}

type builderInterface interface {
	GetServicesToBuild(ctx context.Context, manifest *model.Manifest, svcToDeploy []string) ([]string, error)
	Build(ctx context.Context, options *types.BuildOptions) error
}

func (bc buildCtrl) buildImageIfNecessary(ctx context.Context, manifest *model.Manifest) error {
	oktetoLog.Debug("checking if destroy.image is already built")
	imageToBuild := manifest.Destroy.Image

	reg := regexp.MustCompile(`OKTETO_BUILD_(\w+)_`)
	matches := reg.FindStringSubmatch(imageToBuild)
	foundMatches := 2
	if len(matches) == 0 {
		oktetoLog.Debugf("image '%s' is not an okteto build variable", imageToBuild)
		return nil
	}

	sanitisedToUnsanitised := map[string]string{}
	for buildSvc := range manifest.Build {
		sanitizedSvc := strings.ToUpper(strings.ReplaceAll(buildSvc, "-", "_"))
		sanitisedToUnsanitised[sanitizedSvc] = buildSvc
	}
	if len(matches) == foundMatches {
		sanitisedName := matches[1]
		svc, ok := sanitisedToUnsanitised[sanitisedName]
		if !ok {
			oktetoLog.Infof("image is not defined in build section: %s", imageToBuild)
			return nil
		}
		svcsToBuild, err := bc.builder.GetServicesToBuild(ctx, manifest, []string{svc})
		if err != nil {
			return fmt.Errorf("error getting services to build: %w", err)
		}
		if len(svcsToBuild) == 0 {
			oktetoLog.Infof("image is already built: %s", imageToBuild)
			return nil
		}
		buildOptions := &types.BuildOptions{
			EnableStages: true,
			Manifest:     manifest,
			CommandArgs:  svcsToBuild,
		}

		if errBuild := bc.builder.Build(ctx, buildOptions); errBuild != nil {
			return fmt.Errorf("error building images: %w", errBuild)
		}
	}
	return nil
}
