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

	"github.com/okteto/okteto/internal/test"
	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/okteto/okteto/pkg/vars"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type fakeVarManager struct{}

func (*fakeVarManager) MaskVar(string)                     {}
func (*fakeVarManager) WarningLogf(string, ...interface{}) {}

func newFakeContextCommand(c *client.FakeOktetoClient, user *types.User, fakeObjects []runtime.Object) *Command {
	return &Command{
		K8sClientProvider:    test.NewFakeK8sProvider(fakeObjects...),
		LoginController:      test.NewFakeLoginController(user, nil),
		OktetoClientProvider: client.NewFakeOktetoClientProvider(c),
		OktetoContextWriter:  test.NewFakeOktetoContextWriter(),
		kubetokenController:  newStaticKubetokenController(),
		varManager:           vars.NewVarsManager(&fakeVarManager{}),
	}
}

func Test_createContext(t *testing.T) {
	ctx := context.Background()
	user := &types.User{
		Token: "test",
	}

	var tests = []struct {
		ctxStore         *okteto.ContextStore
		ctxOptions       *Options
		fakeOktetoClient *client.FakeOktetoClient
		name             string
		kubeconfigCtx    test.KubeconfigFields
		fakeObjects      []runtime.Object
		expectedErr      bool
	}{
		{
			name: "change namespace",
			ctxStore: &okteto.ContextStore{
				Contexts: map[string]*okteto.Context{
					"https://okteto.example.com": {},
				},
				CurrentContext: "https://okteto.example.com",
			},
			ctxOptions: &Options{
				IsOkteto:  true,
				Save:      true,
				Context:   "https://okteto-2.example.com",
				Namespace: "test",
			},
			fakeOktetoClient: &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient([]types.Namespace{}, nil),
				Users:           client.NewFakeUsersClient(user),
				Preview:         client.NewFakePreviewClient(&client.FakePreviewResponse{}),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
			},
			kubeconfigCtx: test.KubeconfigFields{
				Name:           []string{"okteto_example_com"},
				Namespace:      []string{"test"},
				CurrentContext: ""},
			expectedErr: false,
		},
		{
			name: "change namespace forbidden",
			ctxStore: &okteto.ContextStore{
				Contexts: map[string]*okteto.Context{
					"https://okteto.example.com": {},
				},
				CurrentContext: "https://okteto.example.com",
			},
			ctxOptions: &Options{
				IsOkteto:             true,
				Save:                 true,
				Context:              "https://okteto.example.com",
				Namespace:            "not-found",
				CheckNamespaceAccess: true,
			},

			kubeconfigCtx: test.KubeconfigFields{
				Name:           []string{"okteto_example_com"},
				Namespace:      []string{"test"},
				CurrentContext: "",
			},
			fakeOktetoClient: &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient([]types.Namespace{}, oktetoErrors.ErrNamespaceNotFound),
				Users:           client.NewFakeUsersClient(user),
				Preview:         client.NewFakePreviewClient(&client.FakePreviewResponse{ErrGetPreview: oktetoErrors.ErrNamespaceNotFound}),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
			},
			expectedErr: true,
		},
		{
			name: "change to personal namespace if namespace is not found",
			ctxStore: &okteto.ContextStore{
				Contexts: map[string]*okteto.Context{
					"https://okteto.example.com": {},
				},
				CurrentContext: "https://okteto.example.com",
			},
			ctxOptions: &Options{
				IsOkteto:  true,
				Save:      true,
				Context:   "https://okteto.example.com",
				Namespace: "not-found",
			},
			kubeconfigCtx: test.KubeconfigFields{
				Name:           []string{"okteto_example_com"},
				Namespace:      []string{"test"},
				CurrentContext: "",
			},
			fakeOktetoClient: &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient([]types.Namespace{}, oktetoErrors.ErrNamespaceNotFound),
				Users:           client.NewFakeUsersClient(user),
				Preview:         client.NewFakePreviewClient(&client.FakePreviewResponse{ErrGetPreview: oktetoErrors.ErrNamespaceNotFound}),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
			},
			expectedErr: false,
		},
		{
			name: "transform k8s to url and create okteto context -> namespace with label",
			ctxStore: &okteto.ContextStore{
				Contexts: map[string]*okteto.Context{},
			},
			ctxOptions: &Options{
				IsOkteto: false,
				Context:  "okteto_example_com",
			},
			kubeconfigCtx: test.KubeconfigFields{
				Name:      []string{"okteto_example_com"},
				Namespace: []string{"test"},
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
			fakeOktetoClient: &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient([]types.Namespace{}, nil),
				Users:           client.NewFakeUsersClient(user),
				Preview:         client.NewFakePreviewClient(&client.FakePreviewResponse{}),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
			},
			expectedErr: false,
		},
		{
			name: "transform k8s to url and create okteto context no namespace found",
			ctxStore: &okteto.ContextStore{
				Contexts: map[string]*okteto.Context{},
			},
			ctxOptions: &Options{
				IsOkteto: false,
				Context:  "okteto_example_com",
			},
			kubeconfigCtx: test.KubeconfigFields{
				Name:      []string{"okteto_example_com"},
				Namespace: []string{"test"},
			},
			fakeOktetoClient: &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient([]types.Namespace{}, nil),
				Users:           client.NewFakeUsersClient(user),
				Preview:         client.NewFakePreviewClient(&client.FakePreviewResponse{}),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
			},
			expectedErr: false,
		},
		{
			name: "transform k8s to url and create okteto context -> namespace without label",
			ctxStore: &okteto.ContextStore{
				Contexts: map[string]*okteto.Context{},
			},
			ctxOptions: &Options{
				IsOkteto: false,
				Context:  "okteto_example_com",
			},
			kubeconfigCtx: test.KubeconfigFields{
				Name:      []string{"okteto_example_com"},
				Namespace: []string{"test"},
			},
			fakeOktetoClient: &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient([]types.Namespace{}, nil),
				Users:           client.NewFakeUsersClient(user),
				Preview:         client.NewFakePreviewClient(&client.FakePreviewResponse{}),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
			},
			expectedErr: false,
		},

		{
			name: "transform k8s to url and create okteto context and no namespace defined",
			ctxStore: &okteto.ContextStore{
				Contexts: map[string]*okteto.Context{},
			},
			ctxOptions: &Options{
				IsOkteto: false,
				Context:  "okteto_example_com",
			},
			kubeconfigCtx: test.KubeconfigFields{
				Name:      []string{"okteto_example_com"},
				Namespace: []string{""},
			},
			fakeOktetoClient: &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient([]types.Namespace{}, nil),
				Users:           client.NewFakeUsersClient(user),
				Preview:         client.NewFakePreviewClient(&client.FakePreviewResponse{}),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
			},
			expectedErr: false,
		},
		{
			name: "transform k8s to url and there is a context",
			ctxStore: &okteto.ContextStore{
				Contexts: map[string]*okteto.Context{
					"https://cloud.okteto.com": {
						Token:    "this is a token",
						IsOkteto: true,
					},
				},
			},
			ctxOptions: &Options{
				IsOkteto: false,
				Context:  "okteto_example_com",
			},
			kubeconfigCtx: test.KubeconfigFields{
				Name:           []string{"okteto_example_com"},
				Namespace:      []string{"test"},
				CurrentContext: "",
			},
			fakeOktetoClient: &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient([]types.Namespace{}, nil),
				Users:           client.NewFakeUsersClient(user),
				Preview:         client.NewFakePreviewClient(&client.FakePreviewResponse{}),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
			},
			expectedErr: false,
		},
		{
			name: "change to available okteto context",
			ctxStore: &okteto.ContextStore{
				Contexts: map[string]*okteto.Context{
					"https://cloud.okteto.com": {
						Token:    "this is a token",
						IsOkteto: true,
					},
				},
			},
			ctxOptions: &Options{
				IsOkteto: true,
				Context:  "cloud.okteto.com",
			},
			kubeconfigCtx: test.KubeconfigFields{

				Name:           []string{"okteto_example_com"},
				Namespace:      []string{"test"},
				CurrentContext: "",
			},
			fakeOktetoClient: &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient([]types.Namespace{}, nil),
				Users:           client.NewFakeUsersClient(user),
				Preview:         client.NewFakePreviewClient(&client.FakePreviewResponse{}),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
			},
			expectedErr: false,
		},
		{
			name: "change to available okteto context",
			ctxStore: &okteto.ContextStore{
				Contexts: map[string]*okteto.Context{
					"https://cloud.okteto.com": {
						Token:    "this is a token",
						IsOkteto: true,
					},
				},
			},
			ctxOptions: &Options{
				IsOkteto: true,
				Context:  "https://cloud.okteto.com",
			},
			kubeconfigCtx: test.KubeconfigFields{
				Name:           []string{"okteto_example_com"},
				Namespace:      []string{"test"},
				CurrentContext: "",
			},
			fakeOktetoClient: &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient([]types.Namespace{}, nil),
				Users:           client.NewFakeUsersClient(user),
				Preview:         client.NewFakePreviewClient(&client.FakePreviewResponse{}),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
			},
			expectedErr: false,
		},
		{
			name: "empty ctx create url",
			ctxStore: &okteto.ContextStore{
				Contexts: make(map[string]*okteto.Context),
			},
			ctxOptions: &Options{
				IsOkteto: true,
				Context:  "https://okteto.example.com",
				Token:    "this is a token",
			},
			kubeconfigCtx: test.KubeconfigFields{
				Name:           []string{"okteto_example_com"},
				Namespace:      []string{"test"},
				CurrentContext: "",
			},
			fakeOktetoClient: &client.FakeOktetoClient{
				Namespace:       client.NewFakeNamespaceClient([]types.Namespace{}, nil),
				Users:           client.NewFakeUsersClient(user),
				Preview:         client.NewFakePreviewClient(&client.FakePreviewResponse{}),
				KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
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

			ctxController := newFakeContextCommand(tt.fakeOktetoClient, user, tt.fakeObjects)
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
		ctxOptions          *Options
		user                *types.User
		fakeOktetoClient    *client.FakeOktetoClient
		name                string
		isAutoAuthTriggered bool
	}{
		{
			name: "okteto context triggers auto auth",
			ctxOptions: &Options{
				IsOkteto:     true,
				Context:      "https://okteto.example.com",
				Token:        "this is a invalid token",
				IsCtxCommand: true,
			},
			user:                user,
			fakeOktetoClient:    fakeOktetoClient,
			isAutoAuthTriggered: true,
		},
		{
			name: "non okteto context command gives unauthorized message",
			ctxOptions: &Options{
				IsOkteto:     true,
				Context:      "https://okteto.example.com",
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
				if err.Error() == fmt.Errorf(oktetoErrors.ErrNotLogged, okteto.GetContext().Name).Error() && tt.isAutoAuthTriggered {
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

	// TODO: add unit-test to cover preview environments access from context
	var tests = []struct {
		ctxOptions       *Options
		fakeOktetoClient *client.FakeOktetoClient
		name             string
		expectedAccess   bool
	}{
		{
			name: "okteto client can access to namespace",
			ctxOptions: &Options{
				IsOkteto:  true,
				Namespace: "test",
			},
			fakeOktetoClient: &client.FakeOktetoClient{
				Namespace: client.NewFakeNamespaceClient(nil, nil),
				Users:     client.NewFakeUsersClient(user, fmt.Errorf("unauthorized. Please run 'okteto context url' and try again")),
				Preview:   client.NewFakePreviewClient(&client.FakePreviewResponse{}),
			},
			expectedAccess: true,
		},
		{
			name: "okteto client can access to preview",
			ctxOptions: &Options{
				IsOkteto:  true,
				Namespace: "test",
			},
			fakeOktetoClient: &client.FakeOktetoClient{
				Namespace: client.NewFakeNamespaceClient(nil, oktetoErrors.ErrNamespaceNotFound),
				Users:     client.NewFakeUsersClient(user, fmt.Errorf("unauthorized. Please run 'okteto context url' and try again")),
				Preview:   client.NewFakePreviewClient(&client.FakePreviewResponse{}),
			},
			expectedAccess: true,
		},
		{
			name: "okteto client cannot access to namespace",
			ctxOptions: &Options{
				IsOkteto:  true,
				Namespace: "non-ccessible-ns",
			},
			fakeOktetoClient: &client.FakeOktetoClient{
				Namespace: client.NewFakeNamespaceClient(nil, oktetoErrors.ErrNamespaceNotFound),
				Users:     client.NewFakeUsersClient(user, fmt.Errorf("unauthorized. Please run 'okteto context url' and try again")),
				Preview: client.NewFakePreviewClient(&client.FakePreviewResponse{
					ErrGetPreview: oktetoErrors.ErrNamespaceNotFound,
				}),
			},
			expectedAccess: false,
		},
		{
			name: "non okteto client can access to namespace",
			ctxOptions: &Options{
				IsOkteto:  false,
				Namespace: "test",
			},
			fakeOktetoClient: &client.FakeOktetoClient{
				Namespace: client.NewFakeNamespaceClient(nil, nil),
				Users:     client.NewFakeUsersClient(user, fmt.Errorf("unauthorized. Please run 'okteto context url' and try again")),
				Preview:   client.NewFakePreviewClient(&client.FakePreviewResponse{}),
			},
			expectedAccess: true,
		},
		{
			name: "non okteto client cannot access to namespace",
			ctxOptions: &Options{
				IsOkteto:  false,
				Namespace: "test",
			},
			fakeOktetoClient: &client.FakeOktetoClient{
				Namespace: client.NewFakeNamespaceClient(nil, nil),
				Users:     client.NewFakeUsersClient(user, fmt.Errorf("unauthorized. Please run 'okteto context url' and try again")),
				Preview:   client.NewFakePreviewClient(&client.FakePreviewResponse{}),
			},
			expectedAccess: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fakeCtxCommand := newFakeContextCommand(tt.fakeOktetoClient, user, []runtime.Object{
				&corev1.Namespace{
					ObjectMeta: v1.ObjectMeta{
						Name: "test",
					},
				},
			})

			currentCtxCommand := *fakeCtxCommand
			if tt.ctxOptions.IsOkteto {
				currentCtxCommand.K8sClientProvider = nil
			} else {
				if !tt.expectedAccess {
					currentCtxCommand = *newFakeContextCommand(tt.fakeOktetoClient, user, []runtime.Object{})
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

	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
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
		output output
		name   string
		input  input
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
			name: "token expired error",
			input: input{
				ns: "test",
				userErr: []error{
					oktetoErrors.ErrTokenExpired,
				},
			},
			output: output{
				uc:  nil,
				err: oktetoErrors.ErrTokenExpired,
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
			cmd := Command{
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
		kubetokenMockResponse client.FakeKubetokenResponse
		userContext           *types.UserContext
		name                  string
		expectedToken         string
		useStaticTokenEnv     bool
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

func Test_loadDotEnv(t *testing.T) {
	type expected struct {
		vars map[string]string
		err  error
	}

	tests := []struct {
		expected expected
		mockfs   func() afero.Fs
		mockEnv  func() *vars.Manager
		name     string
	}{
		{
			name: "missing .env",
			mockfs: func() afero.Fs {
				return afero.NewMemMapFs()
			},
			expected: expected{
				vars: map[string]string{},
				err:  nil,
			},
		},
		{
			name: "empty .env",
			mockfs: func() afero.Fs {
				_ = afero.WriteFile(afero.NewMemMapFs(), ".env", []byte(""), 0644)
				return afero.NewMemMapFs()
			},
			expected: expected{
				vars: map[string]string{},
				err:  nil,
			},
		},
		{
			name: "syntax errors in .env",
			mockfs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, ".env", []byte("@"), 0)
				return fs
			},
			expected: expected{
				vars: map[string]string{},
				err:  fmt.Errorf("error parsing dot env file: unexpected character \"@\" in variable name near \"@\""),
			},
		},
		{
			name: "valid .env with a single var",
			mockfs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, ".env", []byte("TEST=VALUE"), 0644)
				return fs
			},
			expected: expected{
				vars: map[string]string{
					"TEST": "VALUE",
				},
				err: nil,
			},
		},
		{
			name: "valid .env with multiple vars",
			mockfs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, ".env", []byte("TEST=VALUE\nTEST2=VALUE2"), 0644)
				return fs
			},
			expected: expected{
				vars: map[string]string{
					"TEST":  "VALUE",
					"TEST2": "VALUE2",
				},
				err: nil,
			},
		},
		{
			name: "valid .env with multiple vars",
			mockEnv: func() *vars.Manager {
				varManager := vars.NewVarsManager(&fakeVarManager{})
				value4 := vars.Var{Name: "VALUE4", Value: "VALUE4"}
				group := vars.Group{Vars: []vars.Var{value4}}
				varManager.AddGroup(group)
				return varManager
			},
			mockfs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, ".env", []byte("VAR1=VALUE1\nVAR2=VALUE2\nVAR3=${VALUE3:-defaultValue3}\nVAR4=${VALUE4:-defaultValue4}"), 0644)
				return fs
			},
			expected: expected{
				vars: map[string]string{
					"VAR1":   "VALUE1",
					"VAR2":   "VALUE2",
					"VAR3":   "defaultValue3",
					"VAR4":   "VALUE4",
					"VALUE4": "VALUE4",
				},
				err: nil,
			},
		},
		{
			name: "local vars are not overridden",
			mockEnv: func() *vars.Manager {
				varManager := vars.NewVarsManager(&fakeVarManager{})
				value4 := vars.Var{Name: "VAR4", Value: "local"}
				group := vars.Group{Vars: []vars.Var{value4}}
				varManager.AddGroup(group)
				return varManager
			},
			mockfs: func() afero.Fs {
				fs := afero.NewMemMapFs()
				_ = afero.WriteFile(fs, ".env", []byte("VAR4=.env"), 0644)
				return fs
			},
			expected: expected{
				vars: map[string]string{
					"VAR4": "local",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := tt.mockfs()
			varManager := vars.NewVarsManager(&fakeVarManager{})
			if tt.mockEnv != nil {
				varManager = tt.mockEnv()
			}
			cmd := Command{
				varManager: varManager,
			}
			err := cmd.loadDotEnv(fs)
			if tt.expected.err != nil {
				assert.Equal(t, tt.expected.err.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}

			for k, v := range tt.expected.vars {
				actualVar, exists := varManager.Lookup(k)
				assert.Equal(t, v, actualVar)
				assert.Equal(t, true, exists)
			}
		})
	}
}
