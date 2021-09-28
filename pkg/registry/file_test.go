package registry

import (
	"os"
	"testing"

	"github.com/okteto/okteto/pkg/okteto"
)

func Test_translateOktetoRegistryImage(t *testing.T) {
	var tests = []struct {
		name      string
		input     string
		namespace string
		token     *okteto.Token
		want      string
	}{
		{
			name:      "has-okteto-registry-image-dev",
			input:     "FROM okteto.dev/image",
			namespace: "cindy",
			token:     &okteto.Token{Registry: "registry.url"},
			want:      "FROM registry.url/cindy/image",
		},
		{
			name:      "has-okteto-registry-image-global",
			input:     "FROM okteto.global/image",
			namespace: "cindy",
			token:     &okteto.Token{Registry: "registry.url"},
			want:      "FROM registry.url/okteto/image",
		},
		{
			name:      "not-okteto-registry-image",
			input:     "FROM image",
			namespace: "cindy",
			token:     &okteto.Token{Registry: "registry.url"},
			want:      "FROM image",
		},
		{
			name:      "not-image-line",
			input:     "RUN command",
			namespace: "cindy",
			token:     &okteto.Token{Registry: "registry.url"},
			want:      "RUN command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, err := okteto.SetToken(tt.token)
			if err != nil {
				t.Fatal(err)
			}

			defer func() {
				if err := os.RemoveAll(dir); err != nil {
					t.Logf("failed to remove %s: %s", dir, err)
				}
			}()

			if got := translateOktetoRegistryImage(tt.input, tt.namespace); got != tt.want {
				t.Errorf("registry.translateOktetoRegistryImage = %v,  want %v", got, tt.want)
			}
		})
	}
}

func Test_translateCacheHandler(t *testing.T) {
	var tests = []struct {
		name     string
		input    string
		userID   string
		expected string
	}{
		{
			name:     "no-matched",
			input:    "RUN go build",
			userID:   "userid",
			expected: "RUN go build",
		},
		{
			name:     "matched-id-first",
			input:    "RUN --mount=id=1,type=cache,target=/root/.cache/go-build go build",
			userID:   "userid",
			expected: "RUN --mount=id=userid-1,type=cache,target=/root/.cache/go-build go build",
		},
		{
			name:     "matched-id-last",
			input:    "RUN --mount=type=cache,target=/root/.cache/go-build,id=1 go build",
			userID:   "userid",
			expected: "RUN --mount=type=cache,target=/root/.cache/go-build,id=userid-1 go build",
		},
		{
			name:     "matched-noid",
			input:    "RUN --mount=type=cache,target=/root/.cache/go-build go build",
			userID:   "userid",
			expected: "RUN --mount=id=userid,type=cache,target=/root/.cache/go-build go build",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translateCacheHandler(tt.input, tt.userID)
			if tt.expected != result {
				t.Errorf("expected %s got %s in test %s", tt.expected, result, tt.name)
			}
		})
	}
}
