package secrets

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestSecrets_CreateOrUpdate(t *testing.T) {
	ctx := context.Background()
	namespace := "default"
	secretName := "test-secret"

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		StringData: map[string]string{"key": "value"},
	}

	tests := []struct {
		name         string
		existing     []runtime.Object
		modifySecret func(s *v1.Secret)
		mockError    bool
		expectedErr  string
	}{
		{
			name:     "Secret Created Successfully",
			existing: []runtime.Object{},
		},
		{
			name:     "Secret Already Exists - Update Successfully",
			existing: []runtime.Object{secret},
			modifySecret: func(s *v1.Secret) {
				s.StringData["key"] = "newvalue"
			},
		},
		{
			name:        "Create Failed - Unexpected Error",
			existing:    []runtime.Object{},
			mockError:   true,
			expectedErr: "unexpected error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewSimpleClientset(tt.existing...)
			if tt.mockError {
				fakeClient.PrependReactor("create", "secrets", func(action ktesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, nil, errors.New("unexpected error")
				})
			}

			testSecret := secret.DeepCopy()
			if tt.modifySecret != nil {
				tt.modifySecret(testSecret)
			}

			svc := &Secrets{k8sClient: fakeClient}
			err := svc.CreateOrUpdate(ctx, namespace, testSecret)

			if tt.expectedErr != "" {
				assert.EqualError(t, err, tt.expectedErr)
				return
			}
			assert.NoError(t, err)

			resultSecret, err := fakeClient.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
			assert.NoError(t, err)
			assert.Equal(t, testSecret, resultSecret)
		})
	}
}
