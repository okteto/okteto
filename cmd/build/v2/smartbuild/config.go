// Copyright 2025 The Okteto Authors
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

import "github.com/okteto/okteto/pkg/env"

const (
	// OktetoEnableSmartBuildEnvVar represents whether the feature flag to enable smart builds is enabled or not
	OktetoEnableSmartBuildEnvVar = "OKTETO_SMART_BUILDS_ENABLED"

	// ParallelCheckStrategyEnvVar represents whether to use parallel check strategy
	ParallelCheckStrategyEnvVar = "OKTETO_BUILD_CHECK_STRATEGY_PARALLEL"
)

type Config struct {
	isEnabled                 bool
	isSequentialCheckStrategy bool
}

// NewConfig creates a new config
func NewConfig() *Config {
	isEnabled := env.LoadBooleanOrDefault(OktetoEnableSmartBuildEnvVar, true)
	isSequentialCheckStrategy := env.LoadBooleanOrDefault(ParallelCheckStrategyEnvVar, false)
	return &Config{
		isEnabled:                 isEnabled,
		isSequentialCheckStrategy: isSequentialCheckStrategy,
	}
}

// IsEnabled returns true if smart builds are enabled, false otherwise
func (c *Config) IsEnabled() bool {
	return c.isEnabled
}
