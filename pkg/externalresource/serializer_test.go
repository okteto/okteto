package externalresource

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

func TestExternalResource_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expected    ExternalResource
		expectedErr bool
	}{
		{
			name: "invalid external resource: wrong input format",
			data: []byte(`
icon: myicon
notes: /path/to/file
endpoints:
 name: endpoint1
 url: /some/url/1`),
			expectedErr: true,
		},
		{
			name: "invalid external resource: duplicated endpoint names",
			data: []byte(`
icon: myicon
notes: /path/to/file
endpoints:
- name: endpoint1
  url: /some/url/1
- name: endpoint1
  url: /some/url/1`),
			expectedErr: true,
		},
		{
			name: "invalid external resource: no endpoint declared",
			data: []byte(`
icon: myicon
notes: /path/to/file`),
			expectedErr: true,
		},
		{
			name: "valid external resource with property 'notes' empty",
			data: []byte(`
icon: myicon
endpoints:
- name: endpoint1
  url: /some/url/1`),
			expected: ExternalResource{
				Icon: "myicon",
				Endpoints: []ExternalEndpoint{
					{
						Name: "endpoint1",
						Url:  "/some/url/1",
					},
				},
			},
		},
		{
			name: "valid external resource",
			data: []byte(`
icon: myicon
notes: /path/to/file
endpoints:
- name: endpoint1
  url: /some/url/1`),
			expected: ExternalResource{
				Icon: "myicon",
				Notes: &Notes{
					Path: "/path/to/file",
				},
				Endpoints: []ExternalEndpoint{
					{
						Name: "endpoint1",
						Url:  "/some/url/1",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result ExternalResource
			if tt.expectedErr {
				assert.Error(t, yaml.Unmarshal([]byte(tt.data), &result))
			} else {
				assert.NoError(t, yaml.Unmarshal([]byte(tt.data), &result))
				if !reflect.DeepEqual(result, tt.expected) {
					t.Errorf("didn't unmarshal correctly. Actual '%+v', Expected '%+v'", result, tt.expected)
				}
			}
		})
	}
}
