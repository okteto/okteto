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

package test

import (
	"context"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/types"
)

// FakeOktetoBuilder emulates an okteto image builder
type FakeOktetoBuilder struct {
	Err      []error
	Registry *FakeOktetoRegistry
}

// NewFakeOktetoBuilder creates a FakeOktetoBuilder
func NewFakeOktetoBuilder(registry *FakeOktetoRegistry, errors ...error) *FakeOktetoBuilder {
	return &FakeOktetoBuilder{
		Err:      errors,
		Registry: registry,
	}
}

// Run simulates a build
func (fb *FakeOktetoBuilder) Run(_ context.Context, opts *types.BuildOptions) error {
	if fb.Err != nil {
		err := fb.Err[0]
		fb.Err = fb.Err[1:]
		return err
	}

	if opts.Tag != "" {
		if err := fb.Registry.AddImageByOpts(opts); err != nil {
			return err
		}
	}
	return nil
}

// FakeOktetoRegistry emulates an okteto image registry
type FakeOktetoRegistry struct {
	Err      error
	Registry map[string]*FakeImage
}

// FakeImage represents the data from an image
type FakeImage struct {
	Registry string
	Repo     string
	Tag      string
	ImageRef string
	Args     []string
}

// NewFakeOktetoRegistry creates a new registry if not already created
func NewFakeOktetoRegistry(err error) *FakeOktetoRegistry {
	return &FakeOktetoRegistry{
		Err:      err,
		Registry: map[string]*FakeImage{},
	}
}

// AddImageByName adds an image to the registry
func (fb *FakeOktetoRegistry) AddImageByName(images ...string) error {
	for _, image := range images {
		fb.Registry[image] = &FakeImage{}
	}
	return nil
}

// AddImageByOpts adds an image to the registry
func (fb *FakeOktetoRegistry) AddImageByOpts(opts *types.BuildOptions) error {
	fb.Registry[opts.Tag] = &FakeImage{Args: opts.BuildArgs}
	return nil
}

// GetImageTagWithDigest returns the image tag digest
func (fb *FakeOktetoRegistry) GetImageTagWithDigest(imageTag string) (string, error) {
	if _, ok := fb.Registry[imageTag]; !ok {
		return "", oktetoErrors.ErrNotFound
	}
	return imageTag, nil
}
