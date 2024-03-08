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

package v1

import (
	"context"

	"github.com/okteto/okteto/cmd/build/basic"
	buildCmd "github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
)

// OktetoBuilder It is a wrapper of basic.Builder to build an image specified by a Dockerfile. a.k.a. Builder v1
// It mainly extends the basic.Builder with the ability to expand the image tag with the environment variables and
// printing the corresponding output when the build finishes.
type OktetoBuilder struct {
	basic.Builder
}

// NewBuilder creates a new builder wrapping basic.Builder to build images directly from a Dockerfile
func NewBuilder(builder basic.BuildRunner, ioCtrl *io.Controller) *OktetoBuilder {
	return &OktetoBuilder{
		Builder: basic.Builder{
			BuildRunner: builder,
			IoCtrl:      ioCtrl,
		},
	}
}

// NewBuilderFromScratch creates a new okteto builder
func NewBuilderFromScratch(ioCtrl *io.Controller) *OktetoBuilder {
	builder := buildCmd.NewOktetoBuilder(
		&okteto.ContextStateless{
			Store: okteto.GetContextStore(),
		},
		afero.NewOsFs(),
	)
	return NewBuilder(builder, ioCtrl)
}

// IsV1 returns true since it is a builder v1
func (*OktetoBuilder) IsV1() bool {
	return true
}

// Build builds the images defined by a Dockerfile
func (ob *OktetoBuilder) Build(ctx context.Context, options *types.BuildOptions) error {
	return ob.Builder.Build(ctx, options)
}
