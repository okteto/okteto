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

package remote

// TODO: This will probably might be replaced by the basic builder
// OktetoBuilder builds the images
/*type OktetoBuilder struct {
	basic.Builder
}

// NewBuilderFromScratch creates a new okteto builder
func NewBuilderFromScratch(ioCtrl *io.Controller) *OktetoBuilder {
	builder := &buildCmd.OktetoBuilder{
		OktetoContext: &okteto.ContextStateless{
			Store: okteto.GetContextStore(),
		},
		Fs: afero.NewOsFs(),
	}

	return &OktetoBuilder{
		Builder: basic.Builder{
			BuildRunner: builder,
			IoCtrl:      ioCtrl,
		},
	}
}

// Build builds the images defined by a Dockerfile
func (bc *OktetoBuilder) Build(ctx context.Context, options *types.BuildOptions) error {
	return bc.Builder.Build(ctx, options)
}

func (bc *OktetoBuilder) IsV1() bool {
	return false
}*/
