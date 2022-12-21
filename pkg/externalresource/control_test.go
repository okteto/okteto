package externalresource

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/externalresource/k8s"
	"github.com/okteto/okteto/pkg/externalresource/k8s/fake"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
)

type fakeClientProvider struct {
	objects      []runtime.Object
	possibleErrs errs
}

type errs struct {
	providerErr, getErr, createErr, updateErr error
}

func (fcp *fakeClientProvider) provide(_ *rest.Config) (k8s.ExternalResourceV1Interface, error) {
	if fcp.possibleErrs.providerErr != nil {
		return nil, fcp.possibleErrs.providerErr
	}

	possibleERErrors := fake.PossibleERErrors{
		GetErr:    fcp.possibleErrs.getErr,
		CreateErr: fcp.possibleErrs.createErr,
		UpdateErr: fcp.possibleErrs.updateErr,
	}
	return fake.NewFakeExternalResourceV1(possibleERErrors, fcp.objects...), nil
}

func TestDeploy(t *testing.T) {
	ctx := context.Background()
	namespace := "testns"
	var tt = []struct {
		name             string
		expectedErr      bool
		objects          []runtime.Object
		possibleErrs     errs
		externalToDeploy string
		externalInfo     *ExternalResource
	}{
		{
			name:        "provider-error",
			objects:     []runtime.Object{},
			expectedErr: true,
			possibleErrs: errs{
				providerErr: assert.AnError,
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
				Notes:     Notes{},
				Endpoints: []ExternalEndpoint{},
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
				Notes:     Notes{},
				Endpoints: []ExternalEndpoint{},
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
				Notes:     Notes{},
				Endpoints: []ExternalEndpoint{},
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
				Icon:      "",
				Notes:     Notes{},
				Endpoints: []ExternalEndpoint{},
			},
			externalToDeploy: "new",
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
			name:        "external found: update an old external resource",
			expectedErr: false,
			externalInfo: &ExternalResource{
				Icon:      "",
				Notes:     Notes{},
				Endpoints: []ExternalEndpoint{},
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
