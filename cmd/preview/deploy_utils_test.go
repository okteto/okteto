package preview

import (
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
)

func Test_optionsSetup(t *testing.T) {
	ctxUsername := "username"
	okteto.CurrentStore = &okteto.OktetoContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Username: ctxUsername,
			},
		},
	}
	tests := []struct {
		name            string
		expectError     error
		opts            *DeployOptions
		args            []string
		expectedOptions *DeployOptions
		expectedFile    string
	}{
		{
			name: "success-empty-args",
			opts: &DeployOptions{
				scope:      "personal",
				repository: "test-repository",
				branch:     "test-branch",
			},
		},
		{
			name: "success-args",
			args: []string{"preview-name"},
			opts: &DeployOptions{
				scope:      "personal",
				repository: "test-repository",
				branch:     "test-branch",
			},
		},
		{
			name: "success-filename",
			opts: &DeployOptions{
				scope:      "personal",
				repository: "test-repository",
				branch:     "test-branch",
				filename:   "filename-old",
			},
			expectedFile: "filename-old",
		},
		{
			name: "success-filename-file",
			opts: &DeployOptions{
				scope:      "personal",
				repository: "test-repository",
				branch:     "test-branch",
				filename:   "filename-old",
				file:       "file",
			},
			expectedFile: "file",
		},
		{
			name: "get-repository-err",
			opts: &DeployOptions{
				scope:  "personal",
				branch: "test-branch",
			},
			expectError: git.ErrRepositoryNotExists,
		},
		{
			name: "get-branch-err",
			opts: &DeployOptions{
				scope:      "personal",
				repository: "test",
			},
			expectError: git.ErrRepositoryNotExists,
		},
		{
			name: "invalid-scope",
			opts: &DeployOptions{
				scope:      "",
				repository: "test-repository",
				branch:     "test-branch",
			},
			expectError: ErrNotValidPreviewScope,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cwd := t.TempDir()
			err := optionsSetup(cwd, tt.opts, tt.args)
			assert.ErrorIs(t, err, tt.expectError)

			assert.Equal(t, tt.expectedFile, tt.opts.file)
		})
	}

}
