// Copyright 2024 The Okteto Authors
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

package basic

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/types"
	"github.com/okteto/okteto/pkg/validator"
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
	if err := validator.FileArgumentIsNotDir(afero.NewOsFs(), options.File); err != nil {
		return err
	}

	var err error
	options.Tag, err = env.ExpandEnv(options.Tag)
	if err != nil {
		return err
	}

	if err := ob.BuildRunner.Run(ctx, options, ob.IoCtrl); err != nil {
		analytics.TrackBuild(false)
		return err
	}

	if options.Tag == "" {
		ob.IoCtrl.Out().Success("Build succeeded")
		ob.IoCtrl.Out().Infof("Your image won't be pushed. To push your image specify the flag '-t'.")
	} else {
		tags := strings.Split(options.Tag, ",")
		if len(tags) >= 1 {
			displayTag := tags[0]
			ob.IoCtrl.Out().Success("Image '%s' successfully pushed", displayTag)
		}
	}

	analytics.TrackBuild(true)
	return nil
}
