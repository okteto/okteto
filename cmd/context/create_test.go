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
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	authenticationv1 "k8s.io/api/authentication/v1"

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func newFakeContextCommand(c *client.FakeOktetoClient, user *types.User, fakeObjects []runtime.Object) *ContextCommand {
	return &ContextCommand{
		K8sClientProvider:    test.NewFakeK8sProvider(fakeObjects...),
		LoginController:      test.NewFakeLoginController(user, nil),
		OktetoClientProvider: client.NewFakeOktetoClientProvider(c),
		OktetoContextWriter:  test.NewFakeOktetoContextWriter(),
		kubetokenController:  newStaticKubetokenController(),
	}
}

func Test_createContext(t *testing.T) {
	ctx := context.Background()

	var tests = []struct {
		name          string
		ctxStore      *okteto.OktetoContextStore
		ctxOptions    *ContextOptions
		kubeconfigCtx test.KubeconfigFields
		expectedErr   bool
		user          *types.User
		fakeObjects   []runtime.Object
	}{
		{
			name: "change namespace",
			ctxStore: &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"https://okteto.cloud.com": {},
				},
				CurrentContext: "https://okteto.cloud.com",
			},
			ctxOptions: &ContextOptions{
				IsOkteto:  true,
				Save:      true,
				Context:   "https://okteto.cloud.com",
				Namespace: "test",
			},
			user: &types.User{
				Token: "test",
			},
			kubeconfigCtx: test.KubeconfigFields{
				Name:           []string{"cloud_okteto_com"},
				Namespace:      []string{"test"},
				CurrentContext: ""},
			expectedErr: false,
		},
		{
			name: "change namespace forbidden",
			ctxStore: &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"https://okteto.cloud.com": {},
				},
				CurrentContext: "https://okteto.cloud.com",
			},
			ctxOptions: &ContextOptions{
				IsOkteto:             true,
				Save:                 true,
				Context:              "https://okteto.cloud.com",
				Namespace:            "not-found",
				CheckNamespaceAccess: true,
			},
			user: &types.User{
				Token: "test",
			},
			kubeconfigCtx: test.KubeconfigFields{
				Name:           []string{"cloud_okteto_com"},
				Namespace:      []string{"test"},
				CurrentContext: "",
			},
			expectedErr: true,
		},
		{
			name: "change to personal namespace if namespace is not found",
			ctxStore: &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"https://okteto.cloud.com": {},
				},
				CurrentContext: "https://okteto.cloud.com",
			},
			ctxOptions: &ContextOptions{
				IsOkteto:  true,
				Save:      true,
				Context:   "https://okteto.cloud.com",
				Namespace: "not-found",
			},
			user: &types.User{
				Token: "test",
			},
			kubeconfigCtx: test.KubeconfigFields{
				Name:           []string{"cloud_okteto_com"},
				Namespace:      []string{"test"},
				CurrentContext: "",
			},
			expectedErr: false,
		},
		{
			name: "transform k8s to url and create okteto context -> namespace with label",
			ctxStore: &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{},
			},
			ctxOptions: &ContextOptions{
				IsOkteto: false,
				Context:  "cloud_okteto_com",
			},
			kubeconfigCtx: test.KubeconfigFields{
				Name:      []string{"cloud_okteto_com"},
				Namespace: []string{"test"},
			},
			user: &types.User{
				Token: "test",
			},
			fakeObjects: []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: v1.ObjectMeta{
						Name: "test",
						Labels: map[string]string{
							constants.DevLabel: "true",
						},
						Annotations: map[string]string{
							constants.OktetoURLAnnotation: "https://cloud.okteto.com",
						},
					},
				},
			},
			expectedErr: false,
		},
		{
			name: "transform k8s to url and create okteto context no namespace found",
			ctxStore: &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{},
			},
			ctxOptions: &ContextOptions{
				IsOkteto: false,
				Context:  "cloud_okteto_com",
			},
			user: &types.User{
				Token: "test",
			},
			kubeconfigCtx: test.KubeconfigFields{
				Name:      []string{"cloud_okteto_com"},
				Namespace: []string{"test"},
			},
			expectedErr: false,
		},
		{
			name: "transform k8s to url and create okteto context -> namespace without label",
			ctxStore: &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{},
			},
			ctxOptions: &ContextOptions{
				IsOkteto: false,
				Context:  "cloud_okteto_com",
			},
			user: &types.User{
				Token: "test",
			},
			kubeconfigCtx: test.KubeconfigFields{
				Name:      []string{"cloud_okteto_com"},
				Namespace: []string{"test"},
			},
			expectedErr: false,
		},

		{
			name: "transform k8s to url and create okteto context and no namespace defined",
			ctxStore: &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{},
			},
			ctxOptions: &ContextOptions{
				IsOkteto: false,
				Context:  "cloud_okteto_com",
			},
			user: &types.User{
				Token: "test",
			},
			kubeconfigCtx: test.KubeconfigFields{
				Name:      []string{"cloud_okteto_com"},
				Namespace: []string{""},
			},
			expectedErr: false,
		},
		{
			name: "transform k8s to url and there is a context",
			ctxStore: &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"https://cloud.okteto.com": {
						Token:    "this is a token",
						IsOkteto: true,
					},
				},
			},
			user: &types.User{
				Token: "test",
			},
			ctxOptions: &ContextOptions{
				IsOkteto: false,
				Context:  "cloud_okteto_com",
			},
			kubeconfigCtx: test.KubeconfigFields{
				Name:           []string{"cloud_okteto_com"},
				Namespace:      []string{"test"},
				CurrentContext: "",
			},
			expectedErr: false,
		},
		{
			name: "change to available okteto context",
			ctxStore: &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"https://cloud.okteto.com": {
						Token:    "this is a token",
						IsOkteto: true,
					},
				},
			},
			user: &types.User{
				Token: "test",
			},
			ctxOptions: &ContextOptions{
				IsOkteto: true,
				Context:  "cloud.okteto.com",
			},
			kubeconfigCtx: test.KubeconfigFields{

				Name:           []string{"cloud_okteto_com"},
				Namespace:      []string{"test"},
				CurrentContext: "",
			},
			expectedErr: false,
		},
		{
			name: "change to available okteto context",
			ctxStore: &okteto.OktetoContextStore{
				Contexts: map[string]*okteto.OktetoContext{
					"https://cloud.okteto.com": {
						Token:    "this is a token",
						IsOkteto: true,
					},
				},
			},
			user: &types.User{
				Token: "test",
			},
			ctxOptions: &ContextOptions{
				IsOkteto: true,
				Context:  "https://cloud.okteto.com",
			},
			kubeconfigCtx: test.KubeconfigFields{
				Name:           []string{"cloud_okteto_com"},
				Namespace:      []string{"test"},
				CurrentContext: "",
			},
			expectedErr: false,
		},
		{
			name: "empty ctx create url",
			ctxStore: &okteto.OktetoContextStore{
				Contexts: make(map[string]*okteto.OktetoContext),
			},
			ctxOptions: &ContextOptions{
				IsOkteto: true,
				Context:  "https://okteto.cloud.com",
				Token:    "this is a token",
			},
			user: &types.User{
				Token: "test",
			},
			kubeconfigCtx: test.KubeconfigFields{
				Name:           []string{"cloud_okteto_com"},
				Namespace:      []string{"test"},
				CurrentContext: "",
			},
			expectedErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := test.CreateKubeconfig(tt.kubeconfigCtx)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(file)

			fakeOktetoClient := &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient([]types.Namespace{{ID: "test"}}, nil),
				Users:           client.NewFakeUsersClient(tt.user),
				Preview:         client.NewFakePreviewClient(&client.FakePreviewResponse{}),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
			}

			ctxController := newFakeContextCommand(fakeOktetoClient, tt.user, tt.fakeObjects)
			okteto.CurrentStore = tt.ctxStore

			if err := ctxController.UseContext(ctx, tt.ctxOptions); err != nil && !tt.expectedErr {
				t.Fatalf("Not expecting error but got: %s", err.Error())
			} else if tt.expectedErr && err == nil {
				t.Fatal("Not thrown error")
			}
			assert.Equal(t, tt.ctxOptions.Context, okteto.CurrentStore.CurrentContext)
		})
	}
}

func TestAutoAuthWhenNotValidTokenOnlyWhenOktetoContextIsRun(t *testing.T) {
	ctx := context.Background()

	user := &types.User{
		Token: "test",
	}

	fakeOktetoClient := &client.FakeOktetoClient{
		Namespace:       client.NewFakeNamespaceClient([]types.Namespace{{ID: "test"}}, nil),
		Users:           client.NewFakeUsersClient(user, fmt.Errorf("unauthorized. Please run 'okteto context url' and try again")),
		KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
	}

	ctxController := newFakeContextCommand(fakeOktetoClient, user, nil)

	var tests = []struct {
		name                string
		ctxOptions          *ContextOptions
		user                *types.User
		fakeOktetoClient    *client.FakeOktetoClient
		isAutoAuthTriggered bool
	}{
		{
			name: "okteto context triggers auto auth",
			ctxOptions: &ContextOptions{
				IsOkteto:     true,
				Context:      "https://okteto.cloud.com",
				Token:        "this is a invalid token",
				IsCtxCommand: true,
			},
			user:                user,
			fakeOktetoClient:    fakeOktetoClient,
			isAutoAuthTriggered: true,
		},
		{
			name: "non okteto context command gives unauthorized message",
			ctxOptions: &ContextOptions{
				IsOkteto:     true,
				Context:      "https://okteto.cloud.com",
				Token:        "this is a invalid token",
				IsCtxCommand: false,
			},
			user:                user,
			fakeOktetoClient:    fakeOktetoClient,
			isAutoAuthTriggered: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ctxController.initOktetoContext(ctx, tt.ctxOptions)
			if err != nil {
				if err.Error() == fmt.Errorf(oktetoErrors.ErrNotLogged, okteto.Context().Name).Error() && tt.isAutoAuthTriggered {
					t.Fatalf("Not expecting error but got: %s", err.Error())
				}
			}
		})
	}
}

func TestCheckAccessToNamespace(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	user := &types.User{
		Token: "test",
	}

	fakeOktetoClient := &client.FakeOktetoClient{
		Namespace: client.NewFakeNamespaceClient([]types.Namespace{{ID: "test"}}, nil),
		Users:     client.NewFakeUsersClient(user, fmt.Errorf("unauthorized. Please run 'okteto context url' and try again")),
		Preview:   client.NewFakePreviewClient(&client.FakePreviewResponse{}),
	}

	fakeCtxCommand := newFakeContextCommand(fakeOktetoClient, user, []runtime.Object{
		&corev1.Namespace{
			ObjectMeta: v1.ObjectMeta{
				Name: "test",
			},
		},
	})

	// TODO: add unit-test to cover preview environments access from context
	var tests = []struct {
		name           string
		ctxOptions     *ContextOptions
		expectedAccess bool
	}{
		{
			name: "okteto client can access to namespace",
			ctxOptions: &ContextOptions{
				IsOkteto:  true,
				Namespace: "test",
			},
			expectedAccess: true,
		},
		{
			name: "okteto client cannot access to namespace",
			ctxOptions: &ContextOptions{
				IsOkteto:  true,
				Namespace: "non-ccessible-ns",
			},
			expectedAccess: false,
		},
		{
			name: "non okteto client can access to namespace",
			ctxOptions: &ContextOptions{
				IsOkteto:  false,
				Namespace: "test",
			},
			expectedAccess: true,
		},
		{
			name: "non okteto client cannot access to namespace",
			ctxOptions: &ContextOptions{
				IsOkteto:  false,
				Namespace: "test",
			},
			expectedAccess: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			currentCtxCommand := *fakeCtxCommand
			if tt.ctxOptions.IsOkteto {
				currentCtxCommand.K8sClientProvider = nil
			} else {
				if !tt.expectedAccess {
					currentCtxCommand = *newFakeContextCommand(fakeOktetoClient, user, []runtime.Object{})
				}
				currentCtxCommand.OktetoClientProvider = nil
			}
			hasAccess, err := hasAccessToNamespace(ctx, &currentCtxCommand, tt.ctxOptions)
			if err != nil && !strings.Contains(err.Error(), "not found") {
				t.Fatalf("not expecting error but got: %s", err.Error())
			}
			if hasAccess != tt.expectedAccess {
				t.Fatalf("%s fail. expected %t but got: %t", tt.name, tt.expectedAccess, hasAccess)
			}
		})
	}
}

func TestGetUserContext(t *testing.T) {
	ctx := context.Background()

	okteto.CurrentStore = &okteto.OktetoContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				UserID: "test",
			},
		},
	}

	x509Err := errors.New("x509: certificate signed by unknown authority")
	type input struct {
		ns      string
		userErr []error
	}
	type output struct {
		uc  *types.UserContext
		err error
	}
	tt := []struct {
		name   string
		input  input
		output output
	}{
		{
			name: "existing namespace",
			input: input{
				ns: "test",
			},
			output: output{
				uc: &types.UserContext{
					User: types.User{
						Token: "test",
					},
					Credentials: types.Credential{
						Token: "static",
					},
				},
				err: nil,
			},
		},
		{
			name: "unauthorized namespace",
			input: input{
				ns: "test",
				userErr: []error{
					fmt.Errorf("unauthorized. Please run 'okteto context url' and try again"),
				},
			},
			output: output{
				uc:  nil,
				err: oktetoErrors.NotLoggedError{},
			},
		},
		{
			name: "x509 error",
			input: input{
				ns: "test",
				userErr: []error{
					x509Err,
				},
			},
			output: output{
				uc:  nil,
				err: x509Err,
			},
		},
		{
			name: "not found + redirect to personal namespace",
			input: input{
				ns: "test",
				userErr: []error{
					fmt.Errorf("not found"),
				},
			},
			output: output{
				uc: &types.UserContext{
					User: types.User{
						Token: "test",
					},
					Credentials: types.Credential{
						Token: "static",
					},
				},
				err: nil,
			},
		},
		{
			name: "two retries, then success",
			input: input{
				ns: "test",
				userErr: []error{
					fmt.Errorf("first error"),
					fmt.Errorf("second error"),
				},
			},
			output: output{
				uc: &types.UserContext{
					User: types.User{
						Token: "test",
					},
					Credentials: types.Credential{
						Token: "static",
					},
				},
				err: nil,
			},
		},
		{
			name: "max retries exceeded",
			input: input{
				ns: "test",
				userErr: []error{
					assert.AnError,
					assert.AnError,
					assert.AnError,
					assert.AnError,
				},
			},
			output: output{
				uc:  nil,
				err: oktetoErrors.ErrInternalServerError,
			},
		},
	}
	for _, tc := range tt {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			userCtx := &types.UserContext{
				User: types.User{
					Token: "test",
				},
				Credentials: types.Credential{
					Token: "static",
				},
			}

			fakeOktetoClient := &client.FakeOktetoClient{
				Namespace: client.NewFakeNamespaceClient([]types.Namespace{{ID: "test"}}, nil),
				Users:     client.NewFakeUsersClientWithContext(userCtx, tc.input.userErr...),
			}
			cmd := ContextCommand{
				OktetoClientProvider: client.NewFakeOktetoClientProvider(fakeOktetoClient),
				OktetoContextWriter:  test.NewFakeOktetoContextWriter(),
			}
			uc, err := cmd.getUserContext(ctx, "", tc.input.ns, "")
			assert.ErrorIs(t, tc.output.err, err)
			assert.Equal(t, tc.output.uc, uc)
		})
	}
}

func Test_replaceCredentialsTokenWithDynamicKubetoken(t *testing.T) {
	tests := []struct {
		name                  string
		userContext           *types.UserContext
		useStaticTokenEnv     bool
		kubetokenMockResponse client.FakeKubetokenResponse
		expectedToken         string
	}{
		{
			name: "dynamic kubetoken not available, falling back to static token",
			userContext: &types.UserContext{
				User: types.User{
					Token:     "test",
					Namespace: "okteto",
				},
				Credentials: types.Credential{
					Token: "static",
				},
			},
			kubetokenMockResponse: client.FakeKubetokenResponse{
				Token: types.KubeTokenResponse{
					TokenRequest: authenticationv1.TokenRequest{
						Status: authenticationv1.TokenRequestStatus{
							Token: "",
						},
					},
				},
				Err: assert.AnError,
			},
			expectedToken: "static",
		},
		{
			name: "dynamic kubetoken returned successfully and takes priority over static token",
			userContext: &types.UserContext{
				User: types.User{
					Token:     "test",
					Namespace: "okteto",
				},
				Credentials: types.Credential{
					Token: "static",
				},
			},
			kubetokenMockResponse: client.FakeKubetokenResponse{
				Token: types.KubeTokenResponse{
					TokenRequest: authenticationv1.TokenRequest{
						Status: authenticationv1.TokenRequestStatus{
							Token: "dynamic-token",
						},
					},
				},
				Err: nil,
			},
			expectedToken: "dynamic-token",
		},
		{
			name: "using feature flag does not update the token",
			userContext: &types.UserContext{
				User: types.User{
					Token:     "test",
					Namespace: "okteto",
				},
				Credentials: types.Credential{
					Token: "static",
				},
			},
			useStaticTokenEnv: true,
			expectedToken:     "static",
		},
		{
			name: "empty namespace does not update token",
			userContext: &types.UserContext{
				User: types.User{
					Token: "test",
				},
				Credentials: types.Credential{
					Token: "static",
				},
			},
			useStaticTokenEnv: true,
			expectedToken:     "static",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(OktetoUseStaticKubetokenEnvVar, strconv.FormatBool(tt.useStaticTokenEnv))

			fakeOktetoClientProvider := client.NewFakeOktetoClientProvider(&client.FakeOktetoClient{
				KubetokenClient: client.NewFakeKubetokenClient(tt.kubetokenMockResponse),
			})

			newDynamicKubetokenController(fakeOktetoClientProvider).updateOktetoContextToken(tt.userContext)
			assert.Equal(t, tt.expectedToken, tt.userContext.Credentials.Token)
		})
	}
}
