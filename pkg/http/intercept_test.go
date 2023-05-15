package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInterceptAppend(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name: "nothing",
		},
		{
			name:     "single item",
			input:    []string{"https://example.com"},
			expected: []string{"example.com"},
		},
		{
			name:     "multiple items",
			input:    []string{"https://example.com", "https://foo.dev"},
			expected: []string{"example.com", "foo.dev"},
		},
		{
			name:     "repeated items",
			input:    []string{"https://example.com/alice", "https://foo.dev", "https://example.com/bob"},
			expected: []string{"example.com", "foo.dev"},
		},
		{
			name:     "multiple items",
			input:    []string{"https://example.com", "https://foo.dev"},
			expected: []string{"example.com", "foo.dev"},
		},
		{
			name:     "an invalid item",
			input:    []string{"https://example.com", "oneTwOthReE", "https://foo.dev"},
			expected: []string{"example.com", "foo.dev"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := Intercept{}
			i.AppendURLs(tt.input...)

			r := []string{}
			for k, _ := range i {
				r = append(r, k)
			}
			assert.ElementsMatch(t, tt.expected, r)
		})
	}
}

func TestInterceptMatch(t *testing.T) {
	tests := []struct {
		name      string
		bootstrap []string
		expected  map[string]bool
	}{
		{
			name:      "multiple items",
			bootstrap: []string{"https://example.com", "https://foo.dev"},
			expected:  map[string]bool{"example.com:443": true, "foo.dev:443": true, "foo.dev:445": true, "missing.port": false, "other.com:443": false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			i := Intercept{}
			i.AppendURLs(tt.bootstrap...)

			for k, v := range tt.expected {
				assert.Equal(t, v, i.ShouldInterceptAddr(k), k)
			}
		})
	}
}
