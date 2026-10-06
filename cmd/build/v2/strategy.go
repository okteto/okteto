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

// buildServiceFn builds a single service
type buildServiceFn func(ctx context.Context, svc *buildTypes.BuildInfo) error

// buildStrategy decides how the services that are not cached are built
type buildStrategy interface {
	Build(ctx context.Context, svcs []*buildTypes.BuildInfo, build buildServiceFn) error
}
