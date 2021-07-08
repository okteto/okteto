// Copyright 2021 The Okteto Authors
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

package client

import (
	"os"
	"sync"
	"time"

	"github.com/okteto/okteto/pkg/log"
)

var timeout time.Duration
var tOnce sync.Once

func getKubernetesTimeout() time.Duration {
	tOnce.Do(func() {
		timeout = 30 * time.Second
		t, ok := os.LookupEnv("OKTETO_KUBERNETES_TIMEOUT")
		if !ok {
			return
		}

		parsed, err := time.ParseDuration(t)
		if err != nil {
			log.Infof("'%s' is not a valid duration, ignoring", t)
			return
		}

		log.Infof("OKTETO_KUBERNETES_TIMEOUT applied: '%s'", parsed.String())
		timeout = parsed
	})

	return timeout
}
