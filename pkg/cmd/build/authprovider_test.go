package build

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_isErrCredentialsHelperNotAccessiblee(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "credential not accessible error ",
			err:      errors.New("error getting credentials: something resolves to executable in current directory (whatever)"),
			expected: true,
		},
		{
			name:     "credential not accessible error",
			err:      errors.New("error getting credentials: foo executable file not found in $PATH (bar)"),
			expected: true,
		},
		{
			name:     "not a credential not accessible error",
			err:      errors.New("error getting credentials: other error message"),
			expected: false,
		},
		{
			name:     "not a credential not accessible error",
			err:      errors.New("error: resolves to executable in current directory"),
			expected: false,
		},
		{
			name:     "not a credential not accessible error",
			err:      errors.New("a totally different error message"),
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, isErrCredentialsHelperNotAccessible(tt.err), tt.expected)
		})
	}
}
