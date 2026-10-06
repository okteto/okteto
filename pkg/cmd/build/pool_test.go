// Copyright 2026 The Okteto Authors
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

package build

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moby/buildkit/client"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// poolFakeConnector is a connector whose Start waits until unblock is closed
type poolFakeConnector struct {
	unblock  chan struct{}
	startErr error
	stops    atomic.Int32
}

func connectedConnector() *poolFakeConnector {
	c := &poolFakeConnector{unblock: make(chan struct{})}
	close(c.unblock)
	return c
}

func stuckConnector() *poolFakeConnector {
	return &poolFakeConnector{unblock: make(chan struct{})}
}

func (f *poolFakeConnector) Start(ctx context.Context) error {
	select {
	case <-f.unblock:
		return f.startErr
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (*poolFakeConnector) WaitUntilIsReady(context.Context) error { return nil }
func (f *poolFakeConnector) Stop()                                { f.stops.Add(1) }
func (*poolFakeConnector) GetBuildkitClient(context.Context) (*client.Client, error) {
	return &client.Client{}, nil
}
func (*poolFakeConnector) GetType() string { return "fake-pool" }

// fakeConnectorFactory returns conns in order, or err when set
type fakeConnectorFactory struct {
	err   error
	conns []*poolFakeConnector
	calls atomic.Int32
}

func (f *fakeConnectorFactory) create(context.Context) (BuildkitConnector, error) {
	n := f.calls.Add(1)
	if f.err != nil {
		return nil, f.err
	}
	return f.conns[n-1], nil
}

func newPoolTestBuilder(conn BuildkitConnector) *OktetoBuilder {
	return NewOktetoBuilder(&mockOktetoContext{}, afero.NewMemMapFs(), io.NewIOController(), conn)
}

func shortContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	t.Cleanup(cancel)
	return ctx
}

func TestWorkerPoolFirstWorkerUsesBuilderConnector(t *testing.T) {
	base := connectedConnector()
	ob := newPoolTestBuilder(base)
	p := newWorkerPool(context.Background(), ob, 4, nil)
	defer p.Close()

	w, err := p.Acquire(context.Background(), false)

	require.NoError(t, err)
	require.Same(t, ob, w)
	require.Same(t, base, w.connector)
}

func TestWorkerPoolDoesNotGrowWithoutDemand(t *testing.T) {
	factory := &fakeConnectorFactory{conns: []*poolFakeConnector{connectedConnector()}}
	p := newWorkerPool(context.Background(), newPoolTestBuilder(connectedConnector()), 4, factory.create)
	defer p.Close()

	w, err := p.Acquire(context.Background(), false)
	require.NoError(t, err)
	p.Release(w)
	_, err = p.Acquire(context.Background(), false)

	require.NoError(t, err)
	require.Equal(t, int32(0), factory.calls.Load())
}

func TestWorkerPoolGrowsWithExtraConnector(t *testing.T) {
	extra := connectedConnector()
	factory := &fakeConnectorFactory{conns: []*poolFakeConnector{extra}}
	p := newWorkerPool(context.Background(), newPoolTestBuilder(connectedConnector()), 4, factory.create)
	defer p.Close()

	_, err := p.Acquire(context.Background(), false)
	require.NoError(t, err)
	w, err := p.Acquire(context.Background(), true)

	require.NoError(t, err)
	require.Same(t, extra, w.connector)
}

func TestWorkerPoolNeverExceedsSize(t *testing.T) {
	factory := &fakeConnectorFactory{conns: []*poolFakeConnector{connectedConnector(), connectedConnector()}}
	p := newWorkerPool(context.Background(), newPoolTestBuilder(connectedConnector()), 2, factory.create)
	defer p.Close()

	_, err := p.Acquire(context.Background(), false)
	require.NoError(t, err)
	_, err = p.Acquire(context.Background(), true)
	require.NoError(t, err)
	_, err = p.Acquire(shortContext(t), true)

	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Equal(t, int32(1), factory.calls.Load())
}

func TestWorkerPoolStuckExtraDoesNotBlockReleasedWorker(t *testing.T) {
	base := connectedConnector()
	factory := &fakeConnectorFactory{conns: []*poolFakeConnector{stuckConnector()}}
	p := newWorkerPool(context.Background(), newPoolTestBuilder(base), 4, factory.create)
	defer p.Close()

	w0, err := p.Acquire(context.Background(), false)
	require.NoError(t, err)
	p.Release(w0)
	w, err := p.Acquire(context.Background(), true)

	require.NoError(t, err)
	require.Same(t, base, w.connector)
}

func TestWorkerPoolFailedExtraIsNotAnError(t *testing.T) {
	base := connectedConnector()
	factory := &fakeConnectorFactory{err: assert.AnError}
	p := newWorkerPool(context.Background(), newPoolTestBuilder(base), 4, factory.create)
	defer p.Close()

	w0, err := p.Acquire(context.Background(), false)
	require.NoError(t, err)
	_, err = p.Acquire(shortContext(t), true)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	p.Release(w0)
	w, err := p.Acquire(context.Background(), true)

	require.NoError(t, err)
	require.Same(t, base, w.connector)
}

func TestWorkerPoolFailedStartIsStoppedOnClose(t *testing.T) {
	extra := connectedConnector()
	extra.startErr = assert.AnError
	factory := &fakeConnectorFactory{conns: []*poolFakeConnector{extra}}
	p := newWorkerPool(context.Background(), newPoolTestBuilder(connectedConnector()), 4, factory.create)

	_, err := p.Acquire(context.Background(), false)
	require.NoError(t, err)
	_, err = p.Acquire(shortContext(t), true)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	p.Close()

	require.Equal(t, int32(1), extra.stops.Load())
}

func TestWorkerPoolCloseStopsExtrasButNotBuilderConnector(t *testing.T) {
	base := connectedConnector()
	extra := connectedConnector()
	factory := &fakeConnectorFactory{conns: []*poolFakeConnector{extra}}
	p := newWorkerPool(context.Background(), newPoolTestBuilder(base), 4, factory.create)

	w0, err := p.Acquire(context.Background(), false)
	require.NoError(t, err)
	w1, err := p.Acquire(context.Background(), true)
	require.NoError(t, err)
	p.Release(w0)
	p.Release(w1)
	p.Close()

	require.Equal(t, int32(0), base.stops.Load())
	require.Equal(t, int32(1), extra.stops.Load())
}

func TestWorkerPoolCloseStopsPendingExtra(t *testing.T) {
	stuck := stuckConnector()
	factory := &fakeConnectorFactory{conns: []*poolFakeConnector{stuck}}
	p := newWorkerPool(context.Background(), newPoolTestBuilder(connectedConnector()), 4, factory.create)

	_, err := p.Acquire(context.Background(), false)
	require.NoError(t, err)
	_, err = p.Acquire(shortContext(t), true)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	p.Close()

	require.Equal(t, int32(1), stuck.stops.Load())
}

func TestWorkerPoolSharesConnectorWithoutFactory(t *testing.T) {
	base := connectedConnector()
	p := newWorkerPool(context.Background(), newPoolTestBuilder(base), 4, nil)
	defer p.Close()

	w0, err := p.Acquire(context.Background(), false)
	require.NoError(t, err)
	w1, err := p.Acquire(context.Background(), true)

	require.NoError(t, err)
	require.Same(t, base, w1.connector)
	require.NotSame(t, w0, w1)
}

func TestWorkerPoolAcquireReturnsContextError(t *testing.T) {
	p := newWorkerPool(context.Background(), newPoolTestBuilder(connectedConnector()), 1, nil)
	defer p.Close()
	_, err := p.Acquire(context.Background(), false)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = p.Acquire(ctx, false)

	require.ErrorIs(t, err, context.Canceled)
}

func TestWorkerPoolConcurrentUse(t *testing.T) {
	factory := &fakeConnectorFactory{conns: []*poolFakeConnector{connectedConnector(), connectedConnector(), connectedConnector()}}
	p := newWorkerPool(context.Background(), newPoolTestBuilder(connectedConnector()), 4, factory.create)
	defer p.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w, err := p.Acquire(context.Background(), true)
			errs <- err
			p.Release(w)
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	require.LessOrEqual(t, factory.calls.Load(), int32(3))
}
