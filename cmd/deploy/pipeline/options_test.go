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

package pipeline

import (
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestOptions(t *testing.T) {
	// Test WithBranch
	options := &Options{}
	WithBranch("main")(options)
	assert.Equal(t, "main", options.Branch, "Branch is not set correctly")

	// Test WithRepository
	WithRepository("github.com/okteto/example")(options)
	assert.Equal(t, "github.com/okteto/example", options.Repository, "Repository is not set correctly")

	// Test WithName
	WithName("my-pipeline")(options)
	assert.Equal(t, "my-pipeline", options.Name, "Name is not set correctly")

	// Test WithNamespace
	WithNamespace("my-namespace")(options)
	assert.Equal(t, "my-namespace", options.Namespace, "Namespace is not set correctly")

	// Test WithFile
	WithFile("pipeline.yml")(options)
	assert.Equal(t, "pipeline.yml", options.File, "File is not set correctly")

	// Test WithVariables
	WithVariables([]string{"VAR1=value1", "VAR2=value2"})(options)
	assert.Equal(t, []string{"VAR1=value1", "VAR2=value2"}, options.Variables, "Variables are not set correctly")

	// Test WithLabels
	WithLabels([]string{"label1", "label2"})(options)
	assert.Equal(t, []string{"label1", "label2"}, options.Labels, "Labels are not set correctly")

	// Test WithTimeout
	WithTimeout(5 * time.Minute)(options)
	assert.Equal(t, 5*time.Minute, options.Timeout, "Timeout is not set correctly")

	// Test WithWait
	WithWait(true)(options)
	assert.Equal(t, true, options.Wait, "Wait is not set correctly")
}

func TestOptionsDefaults(t *testing.T) {
	fakeOkCtxController := fakeOkCtxController{
		ns: "test",
		cfg: &api.Config{
			CurrentContext: "test",
		},
	}

	dir := t.TempDir()

	_, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}

	var tests = []struct {
		name string
		opts []OptFunc
		dir  string
		err  error
	}{
		{
			name: "error on repo",
			opts: []OptFunc{},
			err:  git.ErrRepositoryNotExists,
		},
		{
			name: "error on k8s available",
			opts: []OptFunc{
				WithRepository("github.com/okteto/example"),
			},
			err: errNoK8sCtxAvailable,
		},
		{
			name: "error providing k8sClient",
			opts: []OptFunc{
				WithRepository("github.com/okteto/example"),
				WithK8sProvider(&test.FakeK8sProvider{ErrProvide: assert.AnError}),
			},
			err: assert.AnError,
		},
		{
			name: "error providing k8sClient",
			opts: []OptFunc{
				WithRepository("github.com/okteto/example"),
				WithK8sProvider(&test.FakeK8sProvider{ErrProvide: assert.AnError}),
			},
			err: assert.AnError,
		},
		{
			name: "error providing k8sClient",
			opts: []OptFunc{
				WithRepository("github.com/okteto/example"),
				WithK8sProvider(test.NewFakeK8sProvider()),
			},
			dir: dir,
			err: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := &Options{
				ioCtrl:          io.NewIOController(),
				okCtxController: fakeOkCtxController,
				projectRootPath: tt.dir,
			}
			for _, opt := range tt.opts {
				opt(options)
			}
			err := options.setDefaults()
			assert.ErrorIs(t, err, tt.err, "Error is not set correctly")
		})
	}
}
