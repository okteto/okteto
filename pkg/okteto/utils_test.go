package okteto

import (
	"os"
	"testing"
)

func Test_SetToken(t *testing.T) {
	var tests = []struct {
		name  string
		token *Token
	}{
		{
			name:  "regular-token",
			token: &Token{Username: "cindy"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, err := SetToken(tt.token)
			if err != nil {
				t.Fatal(err)
			}

			defer func() {
				if err := os.RemoveAll(dir); err != nil {
					t.Logf("failed to remove %s: %s", dir, err)
				}
			}()

			if dir == "" {
				t.Errorf("okteto.SetToken = %v, want not-empty dir", dir)
			}

		})
	}
}
