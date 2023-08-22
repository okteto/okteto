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

package context

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/okteto"
)

func Test_deleteContext(t *testing.T) {

	var tests = []struct {
		name         string
		ctxStore     *okteto.OktetoContextStore
		toDelete     []string
		afterContext string
		expectedErr  bool
	}{
		{
			name: "deleting one existing context",
			ctxStore: &okteto.OktetoContextStore{
				CurrentContext: "test",
				Contexts: map[string]*okteto.OktetoContext{
					"test": {},
				},
			},
			toDelete:     []string{"test"},
			afterContext: "",
			expectedErr:  false,
		},
		{
			name: "deleting more than one existing context",
			ctxStore: &okteto.OktetoContextStore{
				CurrentContext: "test1",
				Contexts: map[string]*okteto.OktetoContext{
					"test1": {},
					"test2": {},
				},
			},
			toDelete:     []string{"test1", "test2"},
			afterContext: "",
			expectedErr:  false,
		},
		{
			name: "deleting non existing context",
			ctxStore: &okteto.OktetoContextStore{
				CurrentContext: "test",
				Contexts: map[string]*okteto.OktetoContext{
					"test": {},
				},
			},
			toDelete:     []string{"non-existing-test"},
			afterContext: "test",
			expectedErr:  true,
		},
		{
			name: "deleting one existing and one non existing context",
			ctxStore: &okteto.OktetoContextStore{
				CurrentContext: "test",
				Contexts: map[string]*okteto.OktetoContext{
					"test": {},
				},
			},
			toDelete:     []string{"test", "non-existing-test"},
			afterContext: "",
			expectedErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := test.CreateKubeconfig(test.KubeconfigFields{})
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(file)
			t.Setenv(constants.OktetoHomeEnvVar, filepath.Dir(file))
			okteto.CurrentStore = tt.ctxStore
			if err := Delete(tt.toDelete); err == nil && tt.expectedErr || err != nil && !tt.expectedErr {
				t.Fatal(err)
			}
			if okteto.ContextStore().CurrentContext != tt.afterContext {
				t.Fatal("not delete correctly")
			}
		})
	}
}
