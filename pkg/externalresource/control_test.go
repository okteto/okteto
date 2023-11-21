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

package externalresource

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/externalresource/k8s"
	"github.com/okteto/okteto/pkg/externalresource/k8s/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

type fakeClientProvider struct {
	possibleErrs errs
	objects      []runtime.Object
}

type errs struct {
	providerErr, getErr, createErr, updateErr, listErr error
}

func (fcp *fakeClientProvider) provide(_ *rest.Config) (k8s.ExternalResourceV1Interface, error) {
	if fcp.possibleErrs.providerErr != nil {
		return nil, fcp.possibleErrs.providerErr
	}

	possibleERErrors := fake.PossibleERErrors{
		GetErr:    fcp.possibleErrs.getErr,
		CreateErr: fcp.possibleErrs.createErr,
		UpdateErr: fcp.possibleErrs.updateErr,
		ListErr:   fcp.possibleErrs.listErr,
	}
	return fake.NewFakeExternalResourceV1(possibleERErrors, fcp.objects...), nil
}

func TestDeploy(t *testing.T) {
	ctx := context.Background()
	namespace := "testns"
	var tt = []struct {
		possibleErrs     errs
		externalInfo     *ExternalResource
		name             string
		externalToDeploy string
		objects          []runtime.Object
		expectedErr      bool
	}{
		{
			name:        "provider-error",
			objects:     []runtime.Object{},
			expectedErr: true,
			possibleErrs: errs{
				providerErr: assert.AnError,
			},
			externalInfo: &ExternalResource{
				Icon:      "",
				Notes:     &Notes{},
				Endpoints: []*ExternalEndpoint{},
			},
		},
		{
			name:        "get-error",
			objects:     []runtime.Object{},
			expectedErr: true,
			possibleErrs: errs{
				getErr: assert.AnError,
			},
			externalInfo: &ExternalResource{
				Icon:      "",
				Notes:     &Notes{},
				Endpoints: []*ExternalEndpoint{},
			},
		},
		{
			name:        "create-error",
			objects:     []runtime.Object{},
			expectedErr: true,
			possibleErrs: errs{
				createErr: assert.AnError,
			},
			externalInfo: &ExternalResource{
				Icon:      "",
				Notes:     &Notes{},
				Endpoints: []*ExternalEndpoint{},
			},
		},
		{
			name:        "update-error",
			expectedErr: true,
			possibleErrs: errs{
				updateErr: assert.AnError,
			},
			externalInfo: &ExternalResource{
				Icon:      "",
				Notes:     &Notes{},
				Endpoints: []*ExternalEndpoint{},
			},
			externalToDeploy: "old",
			objects: []runtime.Object{
				&k8s.External{
					ObjectMeta: v1.ObjectMeta{
						Name:      "old",
						Namespace: namespace,
					},
					Spec: k8s.ExternalResourceSpec{
						Endpoints: []k8s.Endpoint{
							{
								Url: "https://test.com",
							},
						},
					},
				},
			},
		},
		{
			name:        "external not found: create a new external resource",
			expectedErr: false,
			externalInfo: &ExternalResource{
				Icon:      "default",
				Notes:     &Notes{},
				Endpoints: []*ExternalEndpoint{},
			},
			externalToDeploy: "new",
			objects: []runtime.Object{
				&k8s.External{
					ObjectMeta: v1.ObjectMeta{
						Name:      "old",
						Namespace: namespace,
					},
					Spec: k8s.ExternalResourceSpec{
						Icon: "default",
						Endpoints: []k8s.Endpoint{
							{
								Url: "https://test.com",
							},
						},
					},
				},
			},
		},
		{
			name:        "external found: update an old external resource",
			expectedErr: false,
			externalInfo: &ExternalResource{
				Icon:      "default",
				Notes:     &Notes{},
				Endpoints: []*ExternalEndpoint{},
			},
			externalToDeploy: "old",
			objects: []runtime.Object{
				&k8s.External{
					ObjectMeta: v1.ObjectMeta{
						Name:      "old",
						Namespace: namespace,
					},
					Spec: k8s.ExternalResourceSpec{
						Icon: "default",
						Endpoints: []k8s.Endpoint{
							{
								Url: "https://test.com",
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := K8sControl{
				ClientProvider: (&fakeClientProvider{
					objects:      tc.objects,
					possibleErrs: tc.possibleErrs,
				}).provide,
				Cfg: nil,
			}
			if tc.expectedErr {
				assert.Error(t, ctrl.Deploy(ctx, tc.externalToDeploy, namespace, tc.externalInfo))
			} else {
				assert.NoError(t, ctrl.Deploy(ctx, tc.externalToDeploy, namespace, tc.externalInfo))
			}
		})
	}
}

func TestList(t *testing.T) {
	ctx := context.Background()
	namespace := "testns"
	var tt = []struct {
		possibleErrs errs
		name         string
		externals    []runtime.Object
		len          int
		expectedErr  bool
	}{
		{
			name: "list all",
			externals: []runtime.Object{
				&k8s.External{
					ObjectMeta: v1.ObjectMeta{
						Name:      "old",
						Namespace: namespace,
					},
					Spec: k8s.ExternalResourceSpec{
						Endpoints: []k8s.Endpoint{
							{
								Url: "https://test.com",
							},
						},
					},
				},
			},
			len: 1,
		},
		{
			name: "list 2 externals",
			externals: []runtime.Object{
				&k8s.External{
					ObjectMeta: v1.ObjectMeta{
						Name:      "old",
						Namespace: namespace,
						Labels: map[string]string{
							"key": "value",
						},
					},
					Spec: k8s.ExternalResourceSpec{
						Endpoints: []k8s.Endpoint{
							{
								Url: "https://test.com",
							},
						},
					},
				},
				&k8s.External{
					ObjectMeta: v1.ObjectMeta{
						Name:      "1",
						Namespace: namespace,
						Labels: map[string]string{
							"key": "value",
						},
					},
					Spec: k8s.ExternalResourceSpec{
						Endpoints: []k8s.Endpoint{
							{
								Url: "https://test.com",
							},
						},
						Notes: &k8s.Notes{
							Path:     "hello.md",
							Markdown: "asdasin",
						},
					},
				},
			},
			len: 2,
		},
		{
			name:        "providing error",
			externals:   []runtime.Object{},
			len:         0,
			expectedErr: true,
			possibleErrs: errs{
				providerErr: assert.AnError,
			},
		},
		{
			name:        "listing error",
			externals:   []runtime.Object{},
			len:         0,
			expectedErr: true,
			possibleErrs: errs{
				listErr: assert.AnError,
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := K8sControl{
				ClientProvider: (&fakeClientProvider{
					objects:      tc.externals,
					possibleErrs: tc.possibleErrs,
				}).provide,
				Cfg: nil,
			}
			result, err := ctrl.List(ctx, namespace, "")
			if tc.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Len(t, result, tc.len)
		})
	}
}

func TestTranslate(t *testing.T) {
	now := time.Now()
	name := "non sanitized name"
	ns := "test-ns"
	externalResource := &ExternalResource{
		Icon: "default",
		Notes: &Notes{
			Path:     "hello.md",
			Markdown: "asdasin",
		},
		Endpoints: []*ExternalEndpoint{
			{
				Name: "lambda",
				Url:  "https://test.com",
			},
			{
				Name: "mongodbatlas",
				Url:  "https://fake-mongodb-test.com",
			},
		},
	}

	expected := &k8s.External{
		TypeMeta: v1.TypeMeta{
			Kind:       k8s.ExternalResourceKind,
			APIVersion: fmt.Sprintf("%s/%s", k8s.GroupName, k8s.GroupVersion),
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      "non-sanitized-name",
			Namespace: ns,
			Annotations: map[string]string{
				constants.LastUpdatedAnnotation: now.UTC().Format(constants.TimeFormat),
			},
			Labels: map[string]string{
				constants.OktetoNamespaceLabel: ns,
			},
		},
		Spec: k8s.ExternalResourceSpec{
			Icon: "default",
			Name: "non-sanitized-name",
			Notes: &k8s.Notes{
				Path:     "hello.md",
				Markdown: "asdasin",
			},
			Endpoints: []k8s.Endpoint{
				{
					Name: "lambda",
					Url:  "https://test.com",
				},
				{
					Name: "mongodbatlas",
					Url:  "https://fake-mongodb-test.com",
				},
			},
		},
	}

	result := translate(name, ns, externalResource, now)

	require.Equal(t, expected, result)
}
