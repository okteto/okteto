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

package context

import (
	"encoding/base64"
	"os"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd/api"
)

func Test_ExecuteUpdateKubeconfig_DisabledKubetoken(t *testing.T) {

	var tests = []struct {
		okClientProvider oktetoClientProvider
		context          *okteto.ContextStore
		name             string
		kubeconfigCtx    test.KubeconfigFields
	}{
		{
			name: "change current ctx",
			kubeconfigCtx: test.KubeconfigFields{
				Name:           []string{"test", "to-change"},
				Namespace:      []string{"test", "test"},
				CurrentContext: "test",
			},
			context: &okteto.ContextStore{
				CurrentContext: "to-change",
				Contexts: map[string]*okteto.Context{
					"to-change": {
						Namespace: "test",
						Cfg: &api.Config{
							CurrentContext: "to-change",
							Contexts: map[string]*api.Context{
								"to-change": {
									Namespace: "test",
								},
							},
						},
					},
				},
			},
			okClientProvider: client.NewFakeOktetoClientProvider(
				&client.FakeOktetoClient{
					KubetokenClient: client.NewFakeKubetokenClient(
						client.FakeKubetokenResponse{
							Err: assert.AnError,
						},
					),
				},
			),
		},
		{
			name: "change current namespace",
			kubeconfigCtx: test.KubeconfigFields{
				Name:           []string{"to-change"},
				Namespace:      []string{"test"},
				CurrentContext: "to-change",
			},
			context: &okteto.ContextStore{
				CurrentContext: "to-change",
				Contexts: map[string]*okteto.Context{
					"to-change": {
						Namespace: "to-change",
						Cfg: &api.Config{
							CurrentContext: "to-change",
							Contexts: map[string]*api.Context{
								"to-change": {
									Namespace: "to-change",
								},
							},
						},
					},
				},
			},
			okClientProvider: client.NewFakeOktetoClientProvider(
				&client.FakeOktetoClient{
					KubetokenClient: client.NewFakeKubetokenClient(
						client.FakeKubetokenResponse{
							Err: assert.AnError,
						},
					),
				},
			),
		},
		{
			name:          "create if it doesn't exist",
			kubeconfigCtx: test.KubeconfigFields{},
			context: &okteto.ContextStore{
				CurrentContext: "to-change",
				Contexts: map[string]*okteto.Context{
					"to-change": {
						Namespace: "to-change",
						Cfg: &api.Config{
							CurrentContext: "to-change",
							Contexts: map[string]*api.Context{
								"to-change": {
									Namespace: "to-change",
								},
							},
						},
					},
				},
			},
			okClientProvider: client.NewFakeOktetoClientProvider(
				&client.FakeOktetoClient{
					KubetokenClient: client.NewFakeKubetokenClient(
						client.FakeKubetokenResponse{
							Err: assert.AnError,
						},
					),
				},
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okteto.CurrentStore = tt.context
			file, err := test.CreateKubeconfig(tt.kubeconfigCtx)
			if err != nil {
				assert.NoError(t, err, "error creating temporary kubeconfig")
			}
			defer os.Remove(file)

			okContext := okteto.GetContext()
			kubeconfigPaths := []string{file}

			err = newKubeconfigController(tt.okClientProvider).execute(okContext, kubeconfigPaths)
			assert.NoError(t, err, "error writing kubeconfig")

			cfg := kubeconfig.Get(kubeconfigPaths)
			assert.NotNil(t, cfg, "kubeconfig is nil")
			assert.Equal(t, tt.context.CurrentContext, cfg.CurrentContext, "current context has changed")
			assert.Equal(t, tt.context.Contexts[tt.context.CurrentContext].Namespace, cfg.Contexts[tt.context.CurrentContext].Namespace, "namespace has changed")

		})
	}
}

func Test_ExecuteUpdateKubeconfig_EnabledKubetoken(t *testing.T) {
	var tests = []struct {
		context       *okteto.ContextStore
		name          string
		kubeconfigCtx test.KubeconfigFields
	}{
		{
			name: "change current ctx",
			kubeconfigCtx: test.KubeconfigFields{
				Name:           []string{"test", "to-change"},
				Namespace:      []string{"test", "test"},
				CurrentContext: "test",
			},
			context: &okteto.ContextStore{
				CurrentContext: "to-change",
				Contexts: map[string]*okteto.Context{
					"to-change": {
						Namespace: "test",
						Cfg: &api.Config{
							CurrentContext: "to-change",
							Contexts: map[string]*api.Context{
								"to-change": {
									Namespace: "test",
								},
							},
						},
						IsOkteto: true,
					},
				},
			},
		},
		{
			name: "change current namespace",
			kubeconfigCtx: test.KubeconfigFields{
				Name:           []string{"to-change"},
				Namespace:      []string{"test"},
				CurrentContext: "to-change",
			},
			context: &okteto.ContextStore{
				CurrentContext: "to-change",
				Contexts: map[string]*okteto.Context{
					"to-change": {
						Namespace: "to-change",
						Cfg: &api.Config{
							CurrentContext: "to-change",
							Contexts: map[string]*api.Context{
								"to-change": {
									Namespace: "to-change",
								},
							},
						},
						IsOkteto: true,
					},
				},
			},
		},
		{
			name:          "create if it doesn't exist",
			kubeconfigCtx: test.KubeconfigFields{},
			context: &okteto.ContextStore{
				CurrentContext: "to-change",
				Contexts: map[string]*okteto.Context{
					"to-change": {
						Namespace: "to-change",
						Cfg: &api.Config{
							CurrentContext: "to-change",
							Contexts: map[string]*api.Context{
								"to-change": {
									Namespace: "to-change",
								},
							},
						},
						IsOkteto: true,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(OktetoUseStaticKubetokenEnvVar, "false")

			okClientProvider := client.NewFakeOktetoClientProvider(
				&client.FakeOktetoClient{
					KubetokenClient: client.NewFakeKubetokenClient(
						client.FakeKubetokenResponse{},
					),
				},
			)

			okteto.CurrentStore = tt.context
			file, err := test.CreateKubeconfig(tt.kubeconfigCtx)
			if err != nil {
				assert.NoError(t, err, "error creating temporary kubeconfig")
			}
			defer os.Remove(file)

			okContext := okteto.GetContext()
			kubeconfigPaths := []string{file}

			err = newKubeconfigController(okClientProvider).execute(okContext, kubeconfigPaths)
			assert.NoError(t, err, "error writing kubeconfig")

			cfg := kubeconfig.Get(kubeconfigPaths)
			require.NotNil(t, cfg, "kubeconfig is nil")
			assert.Equal(t, tt.context.CurrentContext, cfg.CurrentContext, "current context has changed")
			assert.Equal(t, tt.context.Contexts[tt.context.CurrentContext].Namespace, cfg.Contexts[tt.context.CurrentContext].Namespace, "namespace has changed")
			require.NotNil(t, cfg.AuthInfos)
			assert.Len(t, cfg.AuthInfos, 1)
			assert.NotNil(t, cfg.AuthInfos[""].Exec)
			assert.Empty(t, cfg.AuthInfos[""].Token)
		})
	}
}

func Test_ExecuteUpdateKubeconfig_With_OktetoUseStaticKubetokenEnvVar(t *testing.T) {
	t.Setenv(OktetoUseStaticKubetokenEnvVar, "true")

	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "ctx-test",
		Contexts: map[string]*okteto.Context{
			"ctx-test": {
				UserID:    "test-user",
				Namespace: "ns-text",
				Cfg: &api.Config{
					AuthInfos: map[string]*api.AuthInfo{
						"test-user": {Token: "test-token", Exec: &api.ExecConfig{Command: "test-cmd"}},
					},
				},
				IsOkteto: true,
			},
		},
	}

	file, err := test.CreateKubeconfig(test.KubeconfigFields{
		Name:           []string{"name-test"},
		Namespace:      []string{"ns-test"},
		CurrentContext: "ctx-test",
	})

	if err != nil {
		assert.NoError(t, err, "error creating temporary kubeconfig")
	}
	defer os.Remove(file)

	okContext := okteto.GetContext()
	kubeconfigPaths := []string{file}
	okClientProvider := client.NewFakeOktetoClientProvider(
		&client.FakeOktetoClient{
			KubetokenClient: client.NewFakeKubetokenClient(
				client.FakeKubetokenResponse{},
			),
		},
	)

	err = newKubeconfigController(okClientProvider).execute(okContext, kubeconfigPaths)
	assert.NoError(t, err, "error writing kubeconfig")

	cfg := kubeconfig.Get(kubeconfigPaths)
	assert.Nil(t, cfg.AuthInfos["test-user"].Exec)
	assert.Equal(t, cfg.AuthInfos["test-user"].Token, "test-token")
}

func Test_ExecuteUpdateKubeconfig_ForNonOktetoContext(t *testing.T) {
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "ctx-test",
		Contexts: map[string]*okteto.Context{
			"ctx-test": {
				UserID:    "test-user",
				Namespace: "ns-text",
				Cfg: &api.Config{
					AuthInfos: map[string]*api.AuthInfo{
						"test-user": {Exec: &api.ExecConfig{Command: "test-cmd"}},
					},
				},
				IsOkteto: false,
			},
		},
	}

	file, err := test.CreateKubeconfig(test.KubeconfigFields{
		Name:           []string{"name-test"},
		Namespace:      []string{"ns-test"},
		CurrentContext: "ctx-test",
	})

	if err != nil {
		assert.NoError(t, err, "error creating temporary kubeconfig")
	}
	defer os.Remove(file)

	okContext := okteto.GetContext()
	kubeconfigPaths := []string{file}

	err = newKubeconfigController(nil).execute(okContext, kubeconfigPaths)
	assert.NoError(t, err, "error writing kubeconfig")

	cfg := kubeconfig.Get(kubeconfigPaths)
	assert.NotNil(t, cfg.AuthInfos["test-user"].Exec)
}

func Test_ExecuteUpdateKubeconfig_WithRightCertificate(t *testing.T) {
	oktetoCert := "okteto-cert"
	oktetoCertBase64 := base64.StdEncoding.EncodeToString([]byte(oktetoCert))
	clusterCert := "k8s-cluster-cert"
	ctxName := "https://test.okteto.dev"
	k8sContextName := "test_okteto_dev"
	var tests = []struct {
		name         string
		context      *okteto.ContextStore
		expectedCert string
	}{
		{
			name: "use cluster certificate when insecure and kubernetes api is not exposed behind our ingress controller",
			context: &okteto.ContextStore{
				CurrentContext: ctxName,
				Contexts: map[string]*okteto.Context{
					ctxName: {
						Name:      ctxName,
						Registry:  "registry.okteto.dev",
						Namespace: ctxName,
						Cfg: &api.Config{
							CurrentContext: k8sContextName,
							Contexts: map[string]*api.Context{
								k8sContextName: {
									Namespace: ctxName,
								},
							},
							Clusters: map[string]*api.Cluster{
								k8sContextName: {
									CertificateAuthorityData: []byte(clusterCert),
									Server:                   "1.2.3.4",
								},
							},
						},
						Certificate:        oktetoCertBase64,
						IsStoredAsInsecure: true,
						IsOkteto:           true,
					},
				},
			},
			expectedCert: clusterCert,
		},
		{
			name: "use cluster certificate when not insecure and kubernetes api is not exposed behind our ingress controller",
			context: &okteto.ContextStore{
				CurrentContext: ctxName,
				Contexts: map[string]*okteto.Context{
					ctxName: {
						Name:      ctxName,
						Registry:  "registry.okteto.dev",
						Namespace: ctxName,
						Cfg: &api.Config{
							CurrentContext: k8sContextName,
							Contexts: map[string]*api.Context{
								k8sContextName: {
									Namespace: ctxName,
								},
							},
							Clusters: map[string]*api.Cluster{
								k8sContextName: {
									CertificateAuthorityData: []byte(clusterCert),
									Server:                   "1.2.3.4",
								},
							},
						},
						Certificate:        oktetoCertBase64,
						IsStoredAsInsecure: false,
						IsOkteto:           true,
					},
				},
			},
			expectedCert: clusterCert,
		},
		{
			name: "use cluster certificate when not insecure and kubernetes api is exposed behind our ingress controller",
			context: &okteto.ContextStore{
				CurrentContext: ctxName,
				Contexts: map[string]*okteto.Context{
					ctxName: {
						Name:      ctxName,
						Registry:  "registry.okteto.dev",
						Namespace: ctxName,
						Cfg: &api.Config{
							CurrentContext: k8sContextName,
							Contexts: map[string]*api.Context{
								k8sContextName: {
									Namespace: ctxName,
								},
							},
							Clusters: map[string]*api.Cluster{
								k8sContextName: {
									CertificateAuthorityData: []byte(clusterCert),
									Server:                   "kubernetes.okteto.dev",
								},
							},
						},
						Certificate:        oktetoCertBase64,
						IsStoredAsInsecure: false,
						IsOkteto:           true,
					},
				},
			},
			expectedCert: clusterCert,
		},
		{
			name: "use okteto certificate when insecure and kubernetes api exposed behind our ingress controller",
			context: &okteto.ContextStore{
				CurrentContext: ctxName,
				Contexts: map[string]*okteto.Context{
					ctxName: {
						Name:      ctxName,
						Registry:  "registry.okteto.dev",
						Namespace: ctxName,
						Cfg: &api.Config{
							CurrentContext: k8sContextName,
							Contexts: map[string]*api.Context{
								k8sContextName: {
									Namespace: ctxName,
								},
							},
							Clusters: map[string]*api.Cluster{
								k8sContextName: {
									CertificateAuthorityData: []byte(clusterCert),
									Server:                   "kubernetes.okteto.dev",
								},
							},
						},
						Certificate:        oktetoCertBase64,
						IsStoredAsInsecure: true,
						IsOkteto:           true,
					},
				},
			},
			expectedCert: oktetoCert,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			okClientProvider := client.NewFakeOktetoClientProvider(
				&client.FakeOktetoClient{
					KubetokenClient: client.NewFakeKubetokenClient(
						client.FakeKubetokenResponse{},
					),
				},
			)

			okteto.CurrentStore = tt.context
			kubeconfigFields := test.KubeconfigFields{
				ClusterCert: clusterCert,
			}
			file, err := test.CreateKubeconfig(kubeconfigFields)
			if err != nil {
				assert.NoError(t, err, "error creating temporary kubeconfig")
			}
			defer os.Remove(file)

			okContext := okteto.GetContext()
			kubeconfigPaths := []string{file}

			err = newKubeconfigController(okClientProvider).execute(okContext, kubeconfigPaths)
			assert.NoError(t, err, "error writing kubeconfig")

			cfg := kubeconfig.Get(kubeconfigPaths)
			require.NotNil(t, cfg, "kubeconfig is nil")
			require.Equal(t, tt.expectedCert, string(cfg.Clusters[k8sContextName].CertificateAuthorityData), "certificate in kubeconfig is not correct")

		})
	}
}

func Test_ExecuteUpdateKubeconfig_WithWrongOktetoCertificate(t *testing.T) {
	okClientProvider := client.NewFakeOktetoClientProvider(
		&client.FakeOktetoClient{
			KubetokenClient: client.NewFakeKubetokenClient(
				client.FakeKubetokenResponse{},
			),
		},
	)

	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "https://test.okteto.dev",
		Contexts: map[string]*okteto.Context{
			"https://test.okteto.dev": {
				Name:      "https://test.okteto.dev",
				Namespace: "fake-ns",
				Registry:  "registry.okteto.dev",
				Cfg: &api.Config{
					CurrentContext: "test_okteto_dev",
					Contexts: map[string]*api.Context{
						"test_okteto_dev": {
							Namespace: "fake-ns",
						},
					},
					Clusters: map[string]*api.Cluster{
						"test_okteto_dev": {
							CertificateAuthorityData: []byte("fake-k8s-cert"),
							Server:                   "kubernetes.okteto.dev",
						},
					},
				},
				Certificate:        "invalidcertificatebase64%",
				IsStoredAsInsecure: true,
				IsOkteto:           true,
			},
		},
	}
	file, err := test.CreateKubeconfig(test.KubeconfigFields{})
	if err != nil {
		assert.NoError(t, err, "error creating temporary kubeconfig")
	}
	defer os.Remove(file)

	okContext := okteto.GetContext()
	kubeconfigPaths := []string{file}

	err = newKubeconfigController(okClientProvider).execute(okContext, kubeconfigPaths)
	assert.Error(t, err, "should fail as the okteto certificate is not a valid base64 value")
}
