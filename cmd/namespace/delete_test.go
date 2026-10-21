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

package namespace

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/okteto/okteto/internal/test/client"
	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	personalNamespace = "personal"
	currentNamespace  = "current"
)

type fakeGetResponse struct {
	ns  *types.Namespace
	err error
}

// fakeDeleteNamespaceClient returns the configured Get responses in order for the deleted namespace,
// repeating the last one. Get calls for any other namespace are handled by the embedded fake
type fakeDeleteNamespaceClient struct {
	*client.FakeNamespaceClient
	deletedNs    string
	getResponses []fakeGetResponse
	getCalls     int
}

func (f *fakeDeleteNamespaceClient) Delete(ctx context.Context, namespace string) error {
	f.deletedNs = namespace
	return f.FakeNamespaceClient.Delete(ctx, namespace)
}

func (f *fakeDeleteNamespaceClient) Get(ctx context.Context, namespace string) (*types.Namespace, error) {
	if namespace != f.deletedNs {
		return f.FakeNamespaceClient.Get(ctx, namespace)
	}
	if len(f.getResponses) == 0 {
		return nil, errors.New("unexpected call to Get")
	}
	r := f.getResponses[min(f.getCalls, len(f.getResponses)-1)]
	f.getCalls++
	return r.ns, r.err
}

// blockingStreamClient simulates a stalled logs stream: like the real stream client, it ignores
// the context cancellation and only returns when release is closed
type blockingStreamClient struct {
	*client.FakeStreamClient
	release chan struct{}
}

func (c *blockingStreamClient) DestroyAllLogs(_ context.Context, _ string, _ time.Duration) error {
	<-c.release
	return nil
}

var namespaceDeletedResponse = fakeGetResponse{err: oktetoErrors.ErrNamespaceNotFound}

func newFakeDeleteNamespaceClient(responses ...fakeGetResponse) *fakeDeleteNamespaceClient {
	initNamespaces := []types.Namespace{
		{ID: currentNamespace},
		{ID: personalNamespace},
		{ID: "test-1"},
	}
	return &fakeDeleteNamespaceClient{
		FakeNamespaceClient: client.NewFakeNamespaceClient(initNamespaces, nil),
		getResponses:        responses,
	}
}

func setupDeleteTest(t *testing.T, nsClient types.NamespaceInterface, streamResponse *client.FakeStreamResponse) (*Command, *client.FakeOktetoClient) {
	t.Helper()
	previousPollInterval := pollInterval
	pollInterval = time.Millisecond
	t.Cleanup(func() { pollInterval = previousPollInterval })

	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"test-context": {
				Name:              "test-context",
				Token:             "test-token",
				PersonalNamespace: personalNamespace,
				IsOkteto:          true,
				Namespace:         currentNamespace,
				UserID:            "1",
			},
		},
		CurrentContext: "test-context",
	}
	usr := &types.User{Token: "test-token"}
	fakeOkClient := &client.FakeOktetoClient{
		Namespace:       nsClient,
		Users:           client.NewFakeUsersClient(usr),
		StreamClient:    client.NewFakeStreamClient(streamResponse),
		KubetokenClient: client.NewFakeKubetokenClient(client.FakeKubetokenResponse{}),
	}
	return NewFakeNamespaceCommand(fakeOkClient, usr), fakeOkClient
}

func Test_deleteNamespace(t *testing.T) {
	ctx := context.Background()

	var tests = []struct {
		err            error
		nsClient       *fakeDeleteNamespaceClient
		streamResponse *client.FakeStreamResponse
		name           string
		// toDeleteNs the namespace to delete
		toDeleteNs string
		// finalNs the namespace user should finally be
		finalNs string
	}{
		{
			name:           "delete existing ns, the current one",
			toDeleteNs:     currentNamespace,
			finalNs:        personalNamespace,
			nsClient:       newFakeDeleteNamespaceClient(namespaceDeletedResponse),
			streamResponse: &client.FakeStreamResponse{},
		},
		{
			name:           "delete existing ns, not the current one",
			toDeleteNs:     "test-1",
			finalNs:        currentNamespace,
			nsClient:       newFakeDeleteNamespaceClient(namespaceDeletedResponse),
			streamResponse: &client.FakeStreamResponse{},
		},
		{
			name:           "delete non-existing ns",
			toDeleteNs:     "test-non-existing",
			finalNs:        currentNamespace,
			nsClient:       newFakeDeleteNamespaceClient(),
			streamResponse: &client.FakeStreamResponse{},
		},
		{
			name:       "keeps waiting while the namespace is terminating",
			toDeleteNs: currentNamespace,
			finalNs:    personalNamespace,
			nsClient: newFakeDeleteNamespaceClient(
				fakeGetResponse{ns: &types.Namespace{ID: currentNamespace, Status: "Deleting"}},
				namespaceDeletedResponse,
			),
			streamResponse: &client.FakeStreamResponse{},
		},
		{
			name:       "keeps waiting on transient errors",
			toDeleteNs: currentNamespace,
			finalNs:    personalNamespace,
			nsClient: newFakeDeleteNamespaceClient(
				fakeGetResponse{err: errors.New("connection reset by peer")},
				namespaceDeletedResponse,
			),
			streamResponse: &client.FakeStreamResponse{},
		},
		{
			name:           "delete namespace stream logs failed",
			toDeleteNs:     currentNamespace,
			finalNs:        personalNamespace,
			nsClient:       newFakeDeleteNamespaceClient(namespaceDeletedResponse),
			streamResponse: &client.FakeStreamResponse{StreamErr: assert.AnError},
		},
		{
			name:           "delete namespace failed at job",
			toDeleteNs:     currentNamespace,
			finalNs:        currentNamespace,
			nsClient:       newFakeDeleteNamespaceClient(fakeGetResponse{ns: &types.Namespace{ID: currentNamespace, Status: namespaceStatusDeleteFailed}}),
			streamResponse: &client.FakeStreamResponse{},
			err:            errFailedDeleteNamespace,
		},
		{
			name:           "namespace status reverted to active",
			toDeleteNs:     currentNamespace,
			finalNs:        currentNamespace,
			nsClient:       newFakeDeleteNamespaceClient(fakeGetResponse{ns: &types.Namespace{ID: currentNamespace, Status: namespaceStatusActive}}),
			streamResponse: &client.FakeStreamResponse{},
			err:            errFailedDeleteNamespace,
		},
		{
			name:           "namespace status reverted to sleeping",
			toDeleteNs:     currentNamespace,
			finalNs:        currentNamespace,
			nsClient:       newFakeDeleteNamespaceClient(fakeGetResponse{ns: &types.Namespace{ID: currentNamespace, Status: constants.NamespaceStatusSleeping}}),
			streamResponse: &client.FakeStreamResponse{},
			err:            errFailedDeleteNamespace,
		},
		{
			name:           "error getting namespace status",
			toDeleteNs:     currentNamespace,
			finalNs:        currentNamespace,
			nsClient:       newFakeDeleteNamespaceClient(fakeGetResponse{err: assert.AnError}),
			streamResponse: &client.FakeStreamResponse{},
			err:            assert.AnError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nsFakeCommand, fakeOkClient := setupDeleteTest(t, tt.nsClient, tt.streamResponse)

			err := nsFakeCommand.ExecuteDeleteNamespace(ctx, &DeleteOptions{
				Namespace: tt.toDeleteNs,
				Wait:      true,
				Timeout:   time.Minute,
			})
			require.ErrorIs(t, err, tt.err)
			require.Equal(t, tt.finalNs, okteto.GetContext().Namespace)

			// check namespace has been deleted from list
			ns, err := fakeOkClient.Namespaces().List(ctx)
			require.NoError(t, err)
			for _, n := range ns {
				require.NotEqual(t, n.ID, tt.toDeleteNs)
			}
		})
	}
}

func Test_deleteNamespace_Timeout(t *testing.T) {
	nsClient := newFakeDeleteNamespaceClient(fakeGetResponse{ns: &types.Namespace{ID: currentNamespace, Status: "Deleting"}})
	nsFakeCommand, _ := setupDeleteTest(t, nsClient, &client.FakeStreamResponse{})

	err := nsFakeCommand.ExecuteDeleteNamespace(context.Background(), &DeleteOptions{
		Namespace: currentNamespace,
		Wait:      true,
		Timeout:   50 * time.Millisecond,
	})

	require.ErrorIs(t, err, errDeleteNamespaceTimeout)
	require.Equal(t, currentNamespace, okteto.GetContext().Namespace)
}

func Test_waitForNamespaceDeleted_ContextCanceled(t *testing.T) {
	nsClient := newFakeDeleteNamespaceClient(fakeGetResponse{ns: &types.Namespace{ID: currentNamespace, Status: "Deleting"}})
	nsFakeCommand, _ := setupDeleteTest(t, nsClient, &client.FakeStreamResponse{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := nsFakeCommand.waitForNamespaceDeleted(ctx, currentNamespace, time.Minute)

	require.ErrorIs(t, err, context.Canceled)
}

func Test_deleteNamespace_LogsNeverFinish(t *testing.T) {
	previousGracePeriod := logsGracePeriod
	logsGracePeriod = 10 * time.Millisecond
	t.Cleanup(func() { logsGracePeriod = previousGracePeriod })

	nsClient := newFakeDeleteNamespaceClient(namespaceDeletedResponse)
	nsFakeCommand, fakeOkClient := setupDeleteTest(t, nsClient, &client.FakeStreamResponse{})
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	fakeOkClient.StreamClient = &blockingStreamClient{release: release}

	err := nsFakeCommand.ExecuteDeleteNamespace(context.Background(), &DeleteOptions{
		Namespace: currentNamespace,
		Wait:      true,
		Timeout:   time.Minute,
	})

	require.NoError(t, err)
	require.Equal(t, personalNamespace, okteto.GetContext().Namespace)
}

func Test_deleteNamespace_NoWait(t *testing.T) {
	nsClient := newFakeDeleteNamespaceClient()
	nsFakeCommand, _ := setupDeleteTest(t, nsClient, &client.FakeStreamResponse{})

	err := nsFakeCommand.ExecuteDeleteNamespace(context.Background(), &DeleteOptions{
		Namespace: currentNamespace,
		Wait:      false,
	})

	require.NoError(t, err)
	require.Zero(t, nsClient.getCalls)
	require.Equal(t, personalNamespace, okteto.GetContext().Namespace)
}

func Test_deleteNamespace_FlagDefaults(t *testing.T) {
	cmd := Delete(context.Background(), io.NewIOController())

	wait, err := cmd.Flags().GetBool("wait")
	require.NoError(t, err)
	require.True(t, wait)

	timeout, err := cmd.Flags().GetDuration("timeout")
	require.NoError(t, err)
	require.Equal(t, 5*time.Minute, timeout)
}
