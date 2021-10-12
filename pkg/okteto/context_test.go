package okteto

import (
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/model"
)

func Test_contextWithOktetoTokenEnvVar(t *testing.T) {
	var tests = []struct {
		name         string
		ctxResource  *model.ContextResource
		currentStore *OktetoContextStore
		want         *OktetoContextStore
	}{
		{
			name:        "empty-context",
			ctxResource: &model.ContextResource{Token: "token"},
			currentStore: &OktetoContextStore{
				CurrentContext: "",
				Contexts:       map[string]*OktetoContext{},
			},
			want: &OktetoContextStore{
				CurrentContext: CloudURL,
				Contexts: map[string]*OktetoContext{
					CloudURL: {Name: CloudURL, Token: "token"},
				},
			},
		},
		{
			name:        "with-new-context",
			ctxResource: &model.ContextResource{Token: "token", Context: "context"},
			currentStore: &OktetoContextStore{
				CurrentContext: "",
				Contexts:       map[string]*OktetoContext{},
			},
			want: &OktetoContextStore{
				CurrentContext: "context",
				Contexts: map[string]*OktetoContext{
					"context": {Name: "context", Token: "token"},
				},
			},
		},
		{
			name:        "with-existing-context",
			ctxResource: &model.ContextResource{Token: "token", Context: "context"},
			currentStore: &OktetoContextStore{
				CurrentContext: "context",
				Contexts: map[string]*OktetoContext{
					"context": {Name: "context", Token: "token-old", Namespace: "namespace"},
				},
			},
			want: &OktetoContextStore{
				CurrentContext: "context",
				Contexts: map[string]*OktetoContext{
					"context": {Name: "context", Token: "token", Namespace: "namespace"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			CurrentStore = tt.currentStore
			contextWithOktetoTokenEnvVar(tt.ctxResource)
			if !reflect.DeepEqual(tt.want, CurrentStore) {
				t.Errorf("Test '%s' failed: %+v", tt.name, CurrentStore)
			}
		})
	}
}

func Test_IsOktetoCloud(t *testing.T) {
	var tests = []struct {
		name    string
		context *OktetoContext
		want    bool
	}{
		{name: "is-cloud", context: &OktetoContext{Name: "https://cloud.okteto.com"}, want: true},
		{name: "is-staging", context: &OktetoContext{Name: "https://staging.okteto.dev"}, want: true},
		{name: "is-not-cloud", context: &OktetoContext{Name: "https://cindy.okteto.dev"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			CurrentStore = &OktetoContextStore{
				CurrentContext: "test",
				Contexts: map[string]*OktetoContext{
					"test": tt.context,
				},
			}
			if got := IsOktetoCloud(); got != tt.want {
				t.Errorf("IsOktetoCloud, got %v, want %v", got, tt.want)
			}
		})
	}
}
