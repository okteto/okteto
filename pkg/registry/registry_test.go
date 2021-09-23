package registry

import (
	"fmt"
	"testing"

	"github.com/okteto/okteto/pkg/okteto"
)

func Test_IsGlobalRegistry(t *testing.T) {
	var tests = []struct {
		name string
		tag  string
		want bool
	}{
		{
			name: "is-global-registry",
			tag:  "okteto.global/image",
			want: true,
		},
		{
			name: "is-not-global-registry",
			tag:  "okteto.dev/image",
			want: false,
		},
		{
			name: "is-not-global-registry",
			tag:  "other-image/image",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if got := IsGlobalRegistry(tt.tag); got != tt.want {
				t.Errorf("registry.IsGlobalRegistry = %v, want %v", got, tt.want)
			}

		})
	}
}

func Test_IsDevRegistry(t *testing.T) {
	var tests = []struct {
		name string
		tag  string
		want bool
	}{
		{
			name: "is-dev-registry",
			tag:  "okteto.dev/image",
			want: true,
		},
		{
			name: "is-not-dev-registry",
			tag:  "okteto.global/image",
			want: false,
		},
		{
			name: "is-not-dev-registry",
			tag:  "other-image/image",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if got := IsDevRegistry(tt.tag); got != tt.want {
				t.Errorf("registry.IsDevRegistry = %v, want %v", got, tt.want)
			}

		})
	}
}

func Test_getRegistryURL(t *testing.T) {
	var tests = []struct {
		name string
		tag  string
		want string
	}{
		{
			name: "is-splitted-image-not-docker-io-no-https",
			tag:  "registry.url.net/image/other",
			want: "https://registry.url.net",
		},
		{
			name: "is-splitted-image-docker",
			tag:  "docker.io/image",
			want: "https://registry.hub.docker.com",
		},
		{
			name: "is-splitted-image-docker",
			tag:  "image",
			want: "https://registry.hub.docker.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if got := getRegistryURL(tt.tag); got != tt.want {
				t.Errorf("registry.getRegistryURL = %v, want %v", got, tt.want)
			}

		})
	}
}

func Test_GetRegistryAndRepo(t *testing.T) {
	var tests = []struct {
		name            string
		tag             string
		wantRegistryTag string
		wantImageTag    string
	}{
		{
			name:            "is-splitted-image-not-docker-io",
			tag:             "registry.url.net/image",
			wantRegistryTag: "registry.url.net",
			wantImageTag:    "image",
		},
		{
			name:            "is-splitted-image-not-docker-io-double-slash",
			tag:             "registry.url.net/image/other",
			wantRegistryTag: "registry.url.net",
			wantImageTag:    "image/other",
		},
		{
			name:            "is-splitted-image-docker",
			tag:             "docker.io/image",
			wantRegistryTag: "docker.io",
			wantImageTag:    "image",
		},
		{
			name:            "is-splitted-image-docker",
			tag:             "image",
			wantRegistryTag: "docker.io",
			wantImageTag:    "image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if gotRT, gotIT := GetRegistryAndRepo(tt.tag); gotRT != tt.wantRegistryTag || gotIT != tt.wantImageTag {
				t.Errorf("registry.GetRegistryAndRepo = %v, %v, want %v,%v", gotRT, gotIT, tt.wantRegistryTag, tt.wantImageTag)
			}

		})
	}
}

func Test_ExpandOktetoGlobalRegistry(t *testing.T) {
	var tests = []struct {
		name      string
		tag       string
		wantTag   string
		wantError error
	}{
		{
			name:    "can-use-okteto-global-registry",
			tag:     "okteto.global/image",
			wantTag: "%v/okteto/image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := okteto.GetRegistry()
			if err != nil {
				t.Errorf("error: unable to get registry url %v", err)
			}
			tt.wantTag = fmt.Sprintf(tt.wantTag, url)
			if gotTag, gotError := ExpandOktetoGlobalRegistry(tt.tag); gotTag != tt.wantTag || gotError != tt.wantError {
				t.Errorf("registry.ExpandOktetoGlobalRegistry = %v, %v, want %v,%v", gotTag, gotError, tt.wantTag, tt.wantError)
			}

		})
	}
}
