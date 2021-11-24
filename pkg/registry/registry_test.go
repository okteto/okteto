package registry

import (
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

func Test_IsOktetoRegistry(t *testing.T) {
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
			want: true,
		},
		{
			name: "is-not-dev-registry",
			tag:  "other-image/image",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsOktetoRegistry(tt.tag); got != tt.want {
				t.Errorf("registry.IsOktetoRegistry = %v, want %v", got, tt.want)
			}
		})
	}

}

func Test_translateRegistry(t *testing.T) {
	var tests = []struct {
		name         string
		input        string
		registryType string
		namespace    string
		registry     string
		want         string
	}{
		{
			name:         "is-global-registry",
			input:        "okteto.global/image",
			registryType: okteto.GlobalRegistry,
			namespace:    okteto.DefaultGlobalNamespace,
			registry:     "registry.url",
			want:         "registry.url/okteto/image",
		},
		{
			name:         "is-dev-registry",
			input:        "okteto.dev/image",
			registryType: okteto.DevRegistry,
			namespace:    "cindy",
			registry:     "registry.url",
			want:         "registry.url/cindy/image",
		},
		{
			name:         "is-not-okteto-registry",
			input:        "docker.io/image",
			registryType: okteto.DevRegistry,
			namespace:    "cindy",
			registry:     "registry.url",
			want:         "docker.io/image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okteto.CurrentStore = &okteto.OktetoContextStore{
				CurrentContext: "test",
				Contexts: map[string]*okteto.OktetoContext{
					"test": {
						Name:      "test",
						Namespace: tt.namespace,
						UserID:    "user-id",
						Registry:  tt.registry,
					},
				},
			}

			if got := replaceRegistry(tt.input, tt.registryType, tt.namespace); got != tt.want {
				t.Errorf("registry.replaceRegistry = %v, want %v", got, tt.want)
			}
		})
	}

}
