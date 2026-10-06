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

	"github.com/okteto/okteto/pkg/build/buildkit/connector"
)

// WorkerPool hands out builders that can run builds concurrently. Each worker runs one
// build at a time on its own BuildKit connector. Worker 0 is the builder itself; extra
// workers are new builders created lazily, in the background, when the caller asks for
// more. Every worker has its own build metadata.
type WorkerPool struct {
	ctx     context.Context
	base    *OktetoBuilder
	newConn connectorFactory
	cancel  context.CancelCauseFunc

	// available holds the idle workers. Its capacity is size, so sends never block.
	available chan *OktetoBuilder

	extras []BuildkitConnector
	wg     sync.WaitGroup
	mu     sync.Mutex
	size   int
	// total counts the workers created or being created. Failed extras keep their slot
	// so a broken backend is not asked again for new sessions.
	total int
}

// NewWorkerPool returns a pool of at most size workers. Worker 0 is ob itself and its
// connector is never stopped by the pool. Extra workers get their own connector when ob's connector
// supports it (port-forward, in-cluster) and share ob's connector otherwise (ingress).
func (ob *OktetoBuilder) NewWorkerPool(ctx context.Context, size int) *WorkerPool {
	return newWorkerPool(ctx, ob, size, ob.connectorFactory())
}

func newWorkerPool(ctx context.Context, base *OktetoBuilder, size int, newConn connectorFactory) *WorkerPool {
	size = max(size, 1)
	poolCtx, cancel := context.WithCancelCause(ctx)
	p := &WorkerPool{
		ctx:       poolCtx,
		cancel:    cancel,
		base:      base,
		newConn:   newConn,
		size:      size,
		total:     1,
		available: make(chan *OktetoBuilder, size),
	}
	p.available <- base
	return p
}

// Acquire returns an idle worker, waiting until one is available or ctx is done.
// demand is the number of builds that want a worker right now, including the caller.
// While the pool has fewer workers than demand (and than its size), it starts creating
// an extra worker in the background, which becomes available once its connector is
// connected. Acquire never waits for that extra worker specifically.
func (p *WorkerPool) Acquire(ctx context.Context, demand int) (*OktetoBuilder, error) {
	p.grow(demand)
	select {
	case w := <-p.available:
		return w, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Release returns a worker to the pool once its build has finished
func (p *WorkerPool) Release(w *OktetoBuilder) {
	p.available <- w
}

// Close cancels the extra workers that are still connecting and stops the connectors of
// every extra worker. It must be called once all builds have finished. It never stops
// the builder's own connector, which belongs to the caller.
func (p *WorkerPool) Close() {
	// Extra connectors still connecting are not failures: they are just no longer needed
	p.cancel(connector.ErrConnectorNotNeeded)
	p.wg.Wait()

	p.mu.Lock()
	defer p.mu.Unlock()
	for _, conn := range p.extras {
		conn.Stop()
	}
	p.extras = nil
}

// grow reserves a slot for a new worker when demand exceeds the workers created so far,
// and creates it: right away when the connector is shared, in the background when it
// needs its own connector
func (p *WorkerPool) grow(demand int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.total >= min(demand, p.size) || p.ctx.Err() != nil {
		return
	}
	p.total++

	if p.newConn == nil {
		p.available <- p.newWorker(p.base.connector)
		return
	}
	p.wg.Add(1)
	go p.addExtraWorker()
}

// addExtraWorker opens a new connector and makes it available as a worker. A failure is
// not a build error: the pool keeps working with the workers it already has.
func (p *WorkerPool) addExtraWorker() {
	defer p.wg.Done()

	conn, err := p.newConn(p.ctx)
	if err != nil {
		p.base.logger.Logger().Infof("could not create an extra BuildKit connector: %s", err)
		return
	}
	// Track it before starting so Close stops it even if it connects while closing
	p.mu.Lock()
	p.extras = append(p.extras, conn)
	p.mu.Unlock()

	if err := conn.Start(p.ctx); err != nil {
		p.base.logger.Logger().Infof("could not start an extra BuildKit connector: %s", err)
		return
	}
	p.available <- p.newWorker(conn)
}

// newWorker returns a builder that shares the base builder's context, filesystem and
// logger but uses conn and has its own metadata
func (p *WorkerPool) newWorker(conn BuildkitConnector) *OktetoBuilder {
	return NewOktetoBuilder(p.base.OktetoContext, p.base.Fs, p.base.logger, conn)
}
