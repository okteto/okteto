package model

import (
	"os"
	"reflect"
	"testing"
)

func Test_UpdateNamespace(t *testing.T) {
	var tests = []struct {
		name            string
		in              *ContextResource
		namespace       string
		namespaceEnvVar string
		out             *ContextResource
		wantError       bool
	}{
		{
			name:            "all-empty",
			in:              &ContextResource{},
			namespace:       "",
			namespaceEnvVar: "",
			out:             &ContextResource{},
			wantError:       false,
		},
		{
			name:            "namespace-in-manifest-rest-empty",
			in:              &ContextResource{Namespace: "ns-manifest"},
			namespace:       "",
			namespaceEnvVar: "",
			out:             &ContextResource{Namespace: "ns-manifest"},
			wantError:       false,
		},
		{
			name:            "namespace-in-manifest-with-envvar",
			in:              &ContextResource{Namespace: "ns-manifest"},
			namespace:       "",
			namespaceEnvVar: "ns-envvar",
			out:             &ContextResource{Namespace: "ns-manifest"},
			wantError:       false,
		},
		{
			name:            "namespace-in-manifest-with-same-arg",
			in:              &ContextResource{Namespace: "ns-manifest"},
			namespace:       "ns-manifest",
			namespaceEnvVar: "",
			out:             &ContextResource{Namespace: "ns-manifest"},
			wantError:       false,
		},
		{
			name:            "namespace-in-manifest-with-different-arg",
			in:              &ContextResource{Namespace: "ns-manifest"},
			namespace:       "ns-arg",
			namespaceEnvVar: "",
			wantError:       true,
		},
		{
			name:            "no-namespace-in-manifest-with-arg",
			in:              &ContextResource{},
			namespace:       "ns-arg",
			namespaceEnvVar: "",
			out:             &ContextResource{Namespace: "ns-arg"},
			wantError:       false,
		},
		{
			name:            "no-namespace-in-manifest-with-arg-different-than-envvar",
			in:              &ContextResource{},
			namespace:       "ns-arg",
			namespaceEnvVar: "ns-envvar",
			out:             &ContextResource{Namespace: "ns-arg"},
			wantError:       false,
		},
		{
			name:            "no-namespace-in-manifest-with-envvar",
			in:              &ContextResource{},
			namespace:       "",
			namespaceEnvVar: "ns-envvar",
			out:             &ContextResource{Namespace: "ns-envvar"},
			wantError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("OKTETO_NAMESPACE", tt.namespaceEnvVar)
			err := tt.in.UpdateNamespace(tt.namespace)
			if err != nil && !tt.wantError {
				t.Errorf("Test '%s' failed: %+v", tt.name, err)
			}
			if err == nil && tt.wantError {
				t.Errorf("Test '%s' didn't failed", tt.name)
			}
			if err != nil {
				return
			}
			if !reflect.DeepEqual(tt.in, tt.out) {
				t.Errorf("Test '%s' failed: %+v", tt.name, tt.in)
			}
		})
	}
}

func Test_UpdateContext(t *testing.T) {
	var tests = []struct {
		name          string
		in            *ContextResource
		context       string
		contextEnvVar string
		out           *ContextResource
		wantError     bool
	}{
		{
			name:          "all-empty",
			in:            &ContextResource{},
			context:       "",
			contextEnvVar: "",
			out:           &ContextResource{},
			wantError:     false,
		},
		{
			name:          "context-in-manifest-rest-empty",
			in:            &ContextResource{Context: "ctx-manifest"},
			context:       "",
			contextEnvVar: "",
			out:           &ContextResource{Context: "ctx-manifest"},
			wantError:     false,
		},
		{
			name:          "context-in-manifest-with-envvar",
			in:            &ContextResource{Context: "ctx-manifest"},
			context:       "",
			contextEnvVar: "ctx-envvar",
			out:           &ContextResource{Context: "ctx-manifest"},
			wantError:     false,
		},
		{
			name:          "context-in-manifest-with-same-arg",
			in:            &ContextResource{Context: "ctx-manifest"},
			context:       "ctx-manifest",
			contextEnvVar: "",
			out:           &ContextResource{Context: "ctx-manifest"},
			wantError:     false,
		},
		{
			name:          "context-in-manifest-with-different-arg",
			in:            &ContextResource{Context: "ctx-manifest"},
			context:       "ctx-arg",
			contextEnvVar: "",
			wantError:     true,
		},
		{
			name:          "no-context-in-manifest-with-arg",
			in:            &ContextResource{},
			context:       "ctx-arg",
			contextEnvVar: "",
			out:           &ContextResource{Context: "ctx-arg"},
			wantError:     false,
		},
		{
			name:          "no-context-in-manifest-with-arg-different-than-envvar",
			in:            &ContextResource{},
			context:       "ctx-arg",
			contextEnvVar: "ctx-envvar",
			out:           &ContextResource{Context: "ctx-arg"},
			wantError:     false,
		},
		{
			name:          "no-context-in-manifest-with-envvar",
			in:            &ContextResource{},
			context:       "",
			contextEnvVar: "ctx-envvar",
			out:           &ContextResource{Context: "ctx-envvar"},
			wantError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("OKTETO_CONTEXT", tt.contextEnvVar)
			err := tt.in.UpdateContext(tt.context)
			if err != nil && !tt.wantError {
				t.Errorf("Test '%s' failed: %+v", tt.name, err)
			}
			if err == nil && tt.wantError {
				t.Errorf("Test '%s' didn't failed", tt.name)
			}
			if err != nil {
				return
			}
			if !reflect.DeepEqual(tt.in, tt.out) {
				t.Errorf("Test '%s' failed: %+v", tt.name, tt.in)
			}
		})
	}
}
