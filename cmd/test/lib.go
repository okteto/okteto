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
package test

import (
	"os"
	"time"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

func getDefaultTimeout() time.Duration {
	defaultTimeout := 5 * time.Minute
	t := os.Getenv(model.OktetoTimeoutEnvVar)
	if t == "" {
		return defaultTimeout
	}

	parsed, err := time.ParseDuration(t)
	if err != nil {
		oktetoLog.Infof("OKTETO_TIMEOUT value is not a valid duration: %s", t)
		oktetoLog.Infof("timeout fallback to defaultTimeout")
		return defaultTimeout
	}

	return parsed
}
