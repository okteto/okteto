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

package smartbuild

import (
	"testing"

	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/stretchr/testify/assert"
)

type fakeRegistryController struct {
	err              error
	isGlobalRegistry bool
}

func (frc fakeRegistryController) GetDevImageFromGlobal(image string) string { return image }

func (frc fakeRegistryController) IsGlobalRegistry(string) bool { return frc.isGlobalRegistry }
func (frc fakeRegistryController) IsOktetoRegistry(string) bool { return false }
func (fr fakeRegistryController) Clone(from, to string) (string, error) {
	return from, nil
}
func (fr fakeRegistryController) GetImageTagWithDigest(image string) (string, error) {
	return image + "@sha256:fake", nil
}

type fakeHasher struct {
	hash string
}

func (fh fakeHasher) hashWithBuildContext(*build.Info, string) string {
	return fh.hash
}

func TestGetServiceHash(t *testing.T) {
	service := "fake-service"
	sbc := Ctrl{
		ioCtrl: io.NewIOController(),
		hasher: fakeHasher{
			hash: "hash",
		},
	}
	out := sbc.GetBuildHash(&build.Info{}, service)
	assert.Equal(t, "hash", out)
}

func TestGetBuildHash(t *testing.T) {
	service := "fake-service"
	sbc := Ctrl{
		ioCtrl: io.NewIOController(),
		hasher: fakeHasher{
			hash: "hash",
		},
	}
	out := sbc.GetBuildHash(&build.Info{}, service)
	assert.Equal(t, "hash", out)
}

func TestCloneGlobalImageToDev(t *testing.T) {
	type input struct {
		from string
		to   string
	}
	type output struct {
		err      error
		devImage string
	}

	tests := []struct {
		input  input
		name   string
		output output
	}{
		{
			name: "Global Registry",
			input: input{
				from: "okteto.global/myimage",
				to:   "",
			},
			output: output{
				devImage: "okteto.global/myimage",
			},
		},
		{
			name: "Non-Global Registry",
			input: input{
				from: "okteto.dev/myimage",
				to:   "",
			},
			output: output{
				devImage: "okteto.dev/myimage",
				err:      nil,
			},
		},
		{
			name: "Global Registry with image set in buildInfo",
			input: input{
				from: "okteto.global/myimage:sha",
				to:   "okteto.dev/myimage:1.0",
			},
			output: output{
				devImage: "okteto.global/myimage:sha",
			},
		},
		{
			name: "Non-Global Registry with image set in buildInfo",
			input: input{
				from: "okteto.dev/myimage:sha",
				to:   "okteto.dev/myimage:1.0",
			},
			output: output{
				devImage: "okteto.dev/myimage:sha",
				err:      nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := fakeRegistryController{
				isGlobalRegistry: tt.input.from == "okteto.global/myimage",
				err:              tt.output.err,
			}
			ioCtrl := io.NewIOController()
			cloner := NewCloner(registry, ioCtrl)

			devImage, err := cloner.CloneGlobalImageToDev(tt.input.from, tt.input.to)

			assert.Equal(t, tt.output.devImage, devImage)
			assert.Equal(t, tt.output.err, err)
		})
	}
}
