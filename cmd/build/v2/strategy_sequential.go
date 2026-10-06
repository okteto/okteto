// Copyright 2026 The Okteto Authors
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

package v2

import (
	"context"

	buildTypes "github.com/okteto/okteto/cmd/build/v2/types"
)

// sequentialBuildStrategy builds the services one after another, in the given order,
// and stops at the first error
type sequentialBuildStrategy struct{}

// Build builds every service in order and returns the first error
func (sequentialBuildStrategy) Build(ctx context.Context, svcs []*buildTypes.BuildInfo, _ map[string][]string, build buildServiceFn) error {
	for _, svc := range svcs {
		if err := build(ctx, svc, 1); err != nil {
			return err
		}
	}
	return nil
}

// maxConcurrency returns 1: builds run one after another
func (sequentialBuildStrategy) maxConcurrency() int {
	return 1
}
