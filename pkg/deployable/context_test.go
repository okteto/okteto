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

package deployable

import (
	"testing"

	"github.com/okteto/okteto/pkg/okteto"
)

func setFakeOktetoContext(t *testing.T) {
	t.Helper()

	originalStore := okteto.CurrentStore
	t.Cleanup(func() {
		okteto.CurrentStore = originalStore
	})

	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
			"test": {
				Name:      "test",
				Namespace: "test",
				IsOkteto:  true,
				Gateway:   nil,
			},
		},
	}
}
