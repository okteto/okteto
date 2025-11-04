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
	"github.com/okteto/okteto/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type fakeConfigRepo struct {
	err  error
	sha  string
	diff string
}

func (fcr fakeConfigRepo) GetSHA() (string, error)                { return fcr.sha, fcr.err }
func (fcr fakeConfigRepo) GetLatestDirSHA(string) (string, error) { return fcr.sha, fcr.err }
func (fcr fakeConfigRepo) GetDiffHash(string) (string, error)     { return fcr.diff, fcr.err }

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

type fakeImageTagger struct {
	mock.Mock
}

func (fit *fakeImageTagger) GetGlobalTagFromDevIfNeccesary(tags, namespace, registryURL, buildHash string, ic registry.ImageCtrl) string {
	args := fit.Called(tags, namespace, registryURL, buildHash, ic)
	return args.String(0)
}

func (fit *fakeImageTagger) GetImageReferencesForTag(manifestName, svcToBuildName, tag string) []string {
	args := fit.Called(manifestName, svcToBuildName, tag)
	return args.Get(0).([]string)
}

func (fit *fakeImageTagger) GetImageReferencesForDeploy(manifestName, svcToBuildName string) []string {
	args := fit.Called(manifestName, svcToBuildName)
	return args.Get(0).([]string)
}

type fakeImageCtrl struct {
	mock.Mock
}

func (fic *fakeImageCtrl) IsOktetoCluster() bool {
	args := fic.Called()
	return args.Bool(0)
}

func (fic *fakeImageCtrl) GetGlobalNamespace() string {
	args := fic.Called()
	return args.String(0)
}

func (fic *fakeImageCtrl) GetNamespace() string {
	args := fic.Called()
	return args.String(0)
}

func (fic *fakeImageCtrl) GetRegistryURL() string {
	args := fic.Called()
	return args.String(0)
}

func (fic *fakeImageCtrl) GetImageReference(reference string) (registry.OktetoImageReference, error) {
	args := fic.Called(reference)
	return args.Get(0).(registry.OktetoImageReference), args.Error(1)
}

type fakeOktetoRegistry struct {
	mock.Mock
}

func (fr *fakeOktetoRegistry) GetImageReference(reference string) (registry.OktetoImageReference, error) {
	args := fr.Called(reference)
	return args.Get(0).(registry.OktetoImageReference), args.Error(1)
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
