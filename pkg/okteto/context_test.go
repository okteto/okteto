// Copyright 2023 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package okteto

import (
	"context"
	"encoding/base64"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func Test_UrlToKubernetesContext(t *testing.T) {
	var tests = []struct {
		name string
		in   string
		want string
	}{
		{name: "is-url-with-protocol", in: "https://cloud.okteto.com", want: "cloud_okteto_com"},
		{name: "is-k8scontext", in: "minikube", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := UrlToKubernetesContext(tt.in); result != tt.want {
				t.Errorf("Test '%s' failed: %s", tt.name, result)
			}
		})
	}
}

func Test_K8sContextToOktetoUrl(t *testing.T) {
	var tests = []struct {
		name string
		in   string
		want string
	}{
		{name: "is-url", in: CloudURL, want: CloudURL},
		{name: "is-okteto-context", in: "cloud_okteto_com", want: CloudURL},
		{name: "is-empty", in: "", want: ""},
		{name: "is-k8scontext", in: "minikube", want: "minikube"},
	}

	CurrentStore = &ContextStore{
		Contexts: map[string]*Context{CloudURL: {IsOkteto: true}},
	}
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeK8sProvider := test.NewFakeK8sProvider()
			if result := K8sContextToOktetoUrl(ctx, tt.in, "namespace", fakeK8sProvider); result != tt.want {
				t.Errorf("Test '%s' failed: %s", tt.name, result)
			}
		})
	}
}

func Test_IsOktetoCloud(t *testing.T) {
	var tests = []struct {
		context *Context
		name    string
		want    bool
	}{
		{name: "is-cloud", context: &Context{Name: "https://cloud.okteto.com"}, want: true},
		{name: "is-staging", context: &Context{Name: "https://staging.okteto.dev"}, want: true},
		{name: "is-not-cloud", context: &Context{Name: "https://cindy.okteto.dev"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			CurrentStore = &ContextStore{
				CurrentContext: "test",
				Contexts: map[string]*Context{
					"test": tt.context,
				},
			}
			if got := IsOktetoCloud(); got != tt.want {
				t.Errorf("IsOktetoCloud, got %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_RemoveSchema(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "https",
			url:  "https://okteto.dev.com",
			want: "okteto.dev.com",
		},
		{
			name: "non url",
			url:  "minikube",
			want: "minikube",
		},
		{
			name: "http",
			url:  "http://okteto.com",
			want: "okteto.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveSchema(tt.url)
			if result != tt.want {
				t.Fatalf("Expected %s but got %s", tt.want, result)
			}
		})
	}
}

func Test_AddOktetoCredentialsToCfg(t *testing.T) {
	credCert := "credential-certificate"
	oktetoCert := "okteto-certificate"
	oktetoCertBase64 := base64.StdEncoding.EncodeToString([]byte(oktetoCert))
	server := "kubernetes.okteto.dev"
	token := "fake_token"
	ns := "test"
	userName := "fake_username"

	tests := []struct {
		name              string
		credentialCert    string
		expectedCert      string
		isInsecureContext bool
	}{
		{
			name:              "with credential certificate and secure context",
			credentialCert:    credCert,
			isInsecureContext: false,
			expectedCert:      credCert,
		},
		{
			name:              "with credential certificate and insecure context",
			credentialCert:    credCert,
			isInsecureContext: true,
			expectedCert:      credCert,
		},
		{
			name:              "with empty credential certificate and secure context",
			credentialCert:    "",
			isInsecureContext: false,
			expectedCert:      "",
		},
		{
			name:              "with empty credential certificate and insecure context",
			credentialCert:    "",
			isInsecureContext: true,
			expectedCert:      oktetoCert,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &clientcmdapi.Config{
				Clusters:   map[string]*clientcmdapi.Cluster{},
				AuthInfos:  map[string]*clientcmdapi.AuthInfo{},
				Contexts:   map[string]*clientcmdapi.Context{},
				Extensions: nil,
			}
			creds := &types.Credential{
				Certificate: tt.credentialCert,
				Server:      server,
				Token:       token,
			}
			oktetoContext := Context{
				Name:               "https://test.okteto.dev",
				Certificate:        oktetoCertBase64,
				IsStoredAsInsecure: tt.isInsecureContext,
			}
			expectedKubeconfig := clientcmdapi.Config{
				Clusters: map[string]*clientcmdapi.Cluster{
					"test_okteto_dev": {
						Server:                   server,
						CertificateAuthorityData: []byte(tt.expectedCert),
						Extensions:               map[string]runtime.Object{},
					},
				},
				AuthInfos: map[string]*clientcmdapi.AuthInfo{
					userName: {
						Token:                token,
						ImpersonateUserExtra: map[string][]string{},
						Extensions:           map[string]runtime.Object{},
					},
				},
				Contexts: map[string]*clientcmdapi.Context{
					"test_okteto_dev": {
						Extensions: map[string]runtime.Object{
							constants.OktetoExtension: nil,
						},
						Cluster:   "test_okteto_dev",
						AuthInfo:  userName,
						Namespace: ns,
					},
				},
				CurrentContext: "test_okteto_dev",
			}

			err := AddOktetoCredentialsToCfg(cfg, creds, ns, userName, oktetoContext)

			require.NoError(t, err)
			require.Equal(t, expectedKubeconfig, *cfg)
		})
	}
}

func Test_AddOktetoCredentialsToCfgWhenConfigCredentialHasntToBeDone(t *testing.T) {
	t.Setenv(constants.OktetoSkipConfigCredentialsUpdate, "true")
	cfg := &clientcmdapi.Config{
		Clusters:   map[string]*clientcmdapi.Cluster{},
		AuthInfos:  map[string]*clientcmdapi.AuthInfo{},
		Contexts:   map[string]*clientcmdapi.Context{},
		Extensions: nil,
	}
	creds := &types.Credential{
		Certificate: "cert",
		Server:      "k8s server",
		Token:       "fake token",
	}
	oktetoContext := Context{
		Name:               "https://test.okteto.dev",
		Certificate:        "okteto certificate",
		IsStoredAsInsecure: false,
	}

	expectedCfg := clientcmdapi.Config{
		Clusters:   map[string]*clientcmdapi.Cluster{},
		AuthInfos:  map[string]*clientcmdapi.AuthInfo{},
		Contexts:   map[string]*clientcmdapi.Context{},
		Extensions: nil,
	}

	result := AddOktetoCredentialsToCfg(cfg, creds, "ns", "userName", oktetoContext)

	require.NoError(t, result)
	require.Equal(t, expectedCfg, *cfg, "config should not be updated")
}

func Test_AddOktetoCredentialsToCfgWithInvalidOktetoContext(t *testing.T) {
	cfg := &clientcmdapi.Config{
		Clusters:   map[string]*clientcmdapi.Cluster{},
		AuthInfos:  map[string]*clientcmdapi.AuthInfo{},
		Contexts:   map[string]*clientcmdapi.Context{},
		Extensions: nil,
	}
	creds := &types.Credential{
		Certificate: "",
		Server:      "k8s server",
		Token:       "fake token",
	}
	oktetoContext := Context{
		Name:               "https://test.okteto.dev",
		Certificate:        "base64withinvalidchar%",
		IsStoredAsInsecure: true,
	}

	result := AddOktetoCredentialsToCfg(cfg, creds, "ns", "userName", oktetoContext)

	require.Error(t, result)
}

func TestGetContextStoreFromStorePath(t *testing.T) {
	fs := afero.NewOsFs()
	tempDir, err := afero.TempDir(fs, "", "")
	require.NoError(t, err)
	defer func() {
		_ = fs.RemoveAll(tempDir)
	}()

	oktetoHome := filepath.Join(tempDir, "context")
	err = fs.Mkdir(oktetoHome, 0755)
	require.NoError(t, err)

	file, err := fs.Create(filepath.Join(oktetoHome, "config.json"))
	require.NoError(t, err)
	defer func() {
		_ = fs.RemoveAll(tempDir)
	}()

	content := `{"contexts": {}, "current-context": ""}`
	_, err = file.WriteString(content)
	require.NoError(t, err)

	require.NoError(t, err)
	t.Setenv(constants.OktetoFolderEnvVar, tempDir)
	store := GetContextStoreFromStorePath()

	expected := &ContextStore{
		Contexts: make(map[string]*Context),
	}
	require.EqualValues(t, expected, store)
}
