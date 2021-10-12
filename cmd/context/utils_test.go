package context

import (
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func Test_addKubernetesContext(t *testing.T) {
	var tests = []struct {
		name         string
		cfg          *clientcmdapi.Config
		ctxResource  *model.ContextResource
		currentStore *okteto.OktetoContextStore
		wantStore    *okteto.OktetoContextStore
		wantError    bool
	}{
		{
			name:        "nil-cfg",
			ctxResource: &model.ContextResource{Context: "context"},
			wantError:   true,
		},
		{
			name: "not-found",
			cfg: &clientcmdapi.Config{
				Contexts: map[string]*clientcmdapi.Context{},
			},
			ctxResource: &model.ContextResource{Context: "context"},
			wantError:   true,
		},
		{
			name: "found-and-ctxresource-namespace",
			cfg: &clientcmdapi.Config{
				Contexts: map[string]*clientcmdapi.Context{"context": {Namespace: "n-cfg"}},
			},
			ctxResource: &model.ContextResource{Context: "context", Namespace: "n-ctx"},
			currentStore: &okteto.OktetoContextStore{
				CurrentContext: "",
				Contexts:       map[string]*okteto.OktetoContext{},
			},
			wantStore: &okteto.OktetoContextStore{
				CurrentContext: "context",
				Contexts: map[string]*okteto.OktetoContext{
					"context": {Name: "context", Namespace: "n-ctx"},
				},
			},
			wantError: false,
		},
		{
			name: "found-and-cfg-namespace",
			cfg: &clientcmdapi.Config{
				Contexts: map[string]*clientcmdapi.Context{"context": {Namespace: "n-cfg"}},
			},
			ctxResource: &model.ContextResource{Context: "context"},
			currentStore: &okteto.OktetoContextStore{
				CurrentContext: "",
				Contexts:       map[string]*okteto.OktetoContext{},
			},
			wantStore: &okteto.OktetoContextStore{
				CurrentContext: "context",
				Contexts: map[string]*okteto.OktetoContext{
					"context": {Name: "context", Namespace: "n-cfg"},
				},
			},
			wantError: false,
		},
		{
			name: "found-and-default-namespace",
			cfg: &clientcmdapi.Config{
				Contexts: map[string]*clientcmdapi.Context{"context": {}},
			},
			ctxResource: &model.ContextResource{Context: "context"},
			currentStore: &okteto.OktetoContextStore{
				CurrentContext: "",
				Contexts:       map[string]*okteto.OktetoContext{},
			},
			wantStore: &okteto.OktetoContextStore{
				CurrentContext: "context",
				Contexts: map[string]*okteto.OktetoContext{
					"context": {Name: "context", Namespace: "default"},
				},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okteto.CurrentStore = tt.currentStore
			err := addKubernetesContext(tt.cfg, tt.ctxResource)
			if err != nil && !tt.wantError {
				t.Errorf("Test '%s' failed: %+v", tt.name, err)
			}
			if err == nil && tt.wantError {
				t.Errorf("Test '%s' didn't failed", tt.name)
			}
			if err != nil {
				return
			}
			if !reflect.DeepEqual(tt.wantStore, okteto.CurrentStore) {
				t.Errorf("Test '%s' failed: %+v", tt.name, okteto.CurrentStore)
			}
		})
	}
}
