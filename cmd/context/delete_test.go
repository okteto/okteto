// Copyright 2022 The Okteto Authors
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
	"context"
	"os"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/okteto"
)

func Test_deleteContext(t *testing.T) {
	ctx := context.Background()

	var tests = []struct {
		name         string
		ctxStore     *okteto.OktetoContextStore
		toDelete     string
		afterContext string
		expectedErr  bool
	}{
		{
			name: "deleting existing context",
			ctxStore: &okteto.OktetoContextStore{
				CurrentContext: "test",
				Contexts: map[string]*okteto.OktetoContext{
					"test": {},
				},
			},
			toDelete:     "test",
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
			toDelete:     "non-existing-test",
			afterContext: "test",
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
			okteto.CurrentStore = tt.ctxStore
			if err := Delete(ctx, tt.toDelete); err == nil && tt.expectedErr || err != nil && !tt.expectedErr {
				t.Fatal(err)
			}
			if okteto.ContextStore().CurrentContext != tt.afterContext {
				t.Fatal("not delete correctly")
			}
		})
	}
}
