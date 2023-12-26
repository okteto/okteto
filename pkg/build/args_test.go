package build

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func TestArgsUnmarshalling(t *testing.T) {
	tests := []struct {
		env      map[string]string
		name     string
		data     []byte
		expected Args
	}{
		{
			name: "list",
			data: []byte("- KEY=VALUE"),
			expected: Args{
				{
					Name:  "KEY",
					Value: "VALUE",
				},
			},
			env: map[string]string{},
		},
		{
			name: "list with env var set",
			data: []byte("- KEY=${VALUE2}"),
			expected: Args{
				{
					Name:  "KEY",
					Value: "actual-value",
				},
			},
			env: map[string]string{"VALUE2": "actual-value"},
		},
		{
			name: "list with env var unset",
			data: []byte("- KEY=$VALUE"),
			expected: Args{
				{
					Name:  "KEY",
					Value: "$VALUE",
				},
			},
			env: map[string]string{},
		},
		{
			name: "list with multiple env vars",
			data: []byte(`- KEY2=$VALUE2
- KEY=$VALUE
- KEY3=${VALUE3}`),
			expected: Args{
				{
					Name:  "KEY",
					Value: "$VALUE",
				},
				{
					Name:  "KEY2",
					Value: "actual-value-2",
				},
				{
					Name:  "KEY3",
					Value: "actual-value-3",
				},
			},
			env: map[string]string{"VALUE2": "actual-value-2", "VALUE3": "actual-value-3"},
		},
		{
			name: "map",
			data: []byte("KEY: VALUE"),
			expected: Args{
				{
					Name:  "KEY",
					Value: "VALUE",
				},
			},
			env: map[string]string{},
		},
		{
			name: "map with env var",
			data: []byte("KEY: $MYVAR"),
			expected: Args{
				{
					Name:  "KEY",
					Value: "actual-value",
				},
			},
			env: map[string]string{
				"MYVAR": "actual-value",
			},
		},
		{
			name: "same key and value",
			data: []byte("- KEYVALUE"),
			expected: Args{
				{
					Name:  "KEYVALUE",
					Value: "KEYVALUE",
				},
			},
			env: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			var args Args
			if err := yaml.UnmarshalStrict(tt.data, &args); err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, tt.expected, args)
		})
	}
}

func TestSerializeArs(t *testing.T) {
	tests := []struct {
		name     string
		input    Args
		expected []string
	}{
		{
			name:     "no args returns empty list",
			input:    nil,
			expected: []string{},
		},
		{
			name: "args exits, returns expected string list",
			input: Args{
				{
					Name:  "AKEY",
					Value: "AVALUE",
				},
				{
					Name:  "CKEY",
					Value: "CVALUE",
				},
				{
					Name:  "BKEY",
					Value: "BVALUE",
				},
			},
			expected: []string{
				"AKEY=AVALUE",
				"BKEY=BVALUE",
				"CKEY=CVALUE",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SerializeArgs(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
