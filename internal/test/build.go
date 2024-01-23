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

	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/types"
)

// FakeOktetoBuilder emulates an okteto image builder
type FakeOktetoBuilder struct {
	Registry fakeOktetoRegistryInterface
	Err      []error
}

type fakeOktetoRegistryInterface interface {
	AddImageByOpts(opts *types.BuildOptions) error
}

// NewFakeOktetoBuilder creates a FakeOktetoBuilder
func NewFakeOktetoBuilder(registry fakeOktetoRegistryInterface, errors ...error) *FakeOktetoBuilder {
	return &FakeOktetoBuilder{
		Err:      errors,
		Registry: registry,
	}
}

func (fb *FakeOktetoBuilder) GetBuilder() string {
	return "test"
}

// Run simulates a build
func (fb *FakeOktetoBuilder) Run(_ context.Context, opts *types.BuildOptions, _ *io.Controller) error {
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
