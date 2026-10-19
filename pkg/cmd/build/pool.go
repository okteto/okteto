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
	"slices"
	"sync"

	"github.com/okteto/okteto/pkg/build/buildkit/connector"
	"github.com/okteto/okteto/pkg/log/io"
)

// WorkerPool hands out builders that can run builds concurrently. Each worker runs one
// build at a time on its own BuildKit connector. Worker 0 is the builder itself; extra
// workers are new builders created lazily, in the background, when the demand grows.
// Every worker has its own build metadata. Workers are handed out in the order they were
// reserved.
type WorkerPool struct {
	ctx     context.Context
	base    *OktetoBuilder
	newConn connectorFactory
	cancel  context.CancelCauseFunc
	// restoreBaseOutput gives the builder's connector back its output on Close
	restoreBaseOutput func()

	// idle holds the free workers and waiting the reservations still without a worker
	idle    []*OktetoBuilder
	waiting []*WorkerTicket
	extras  []BuildkitConnector
	wg      sync.WaitGroup
	mu      sync.Mutex
	size    int
	// total counts the workers created or being created. Failed extras keep their slot
	// so a broken backend is not asked again for new sessions.
	total int
}

// WorkerTicket is the reservation of a worker of a WorkerPool
type WorkerTicket struct {
	pool   *WorkerPool
	worker chan *OktetoBuilder
}

// outputUser is implemented by connectors that can change where they print, e.g. their
// queue and readiness spinners
type outputUser interface {
	UseOutput(ioCtrl *io.Controller) *io.Controller
}

// NewWorkerPool returns a pool of at most size workers. Worker 0 is ob itself and its
// connector is never stopped by the pool; until Close, it prints nothing, so its spinners
// don't draw over the output of the batch. Extra workers get their own connector (a new
// session for port-forward and in-cluster, a new client for ingress). When ob's connector
// can't open more connections the pool has a single worker: a connector is never shared
// by builds running at the same time.
func (ob *OktetoBuilder) NewWorkerPool(ctx context.Context, size int) *WorkerPool {
	p := newWorkerPool(ctx, ob, size, ob.connectorFactory())
	if conn, ok := ob.connector.(outputUser); ok {
		previous := conn.UseOutput(ob.logger.WithQuietOutput())
		p.restoreBaseOutput = func() { conn.UseOutput(previous) }
	}
	return p
}

func newWorkerPool(ctx context.Context, base *OktetoBuilder, size int, newConn connectorFactory) *WorkerPool {
	if newConn == nil {
		size = 1
	}
	poolCtx, cancel := context.WithCancelCause(ctx)
	return &WorkerPool{
		ctx:     poolCtx,
		cancel:  cancel,
		base:    base,
		newConn: newConn,
		size:    max(size, 1),
		total:   1,
		idle:    []*OktetoBuilder{base},
	}
}

// Reserve reserves a worker without waiting for it. demand is the number of builds that
// want a worker right now, including this one: while the pool has fewer workers than
// demand (and than its size), it creates the missing ones in the background. They become
// available once their connector is connected.
func (p *WorkerPool) Reserve(demand int) *WorkerTicket {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.growLocked(demand)

	t := &WorkerTicket{pool: p, worker: make(chan *OktetoBuilder, 1)}
	if len(p.idle) > 0 && len(p.waiting) == 0 {
		t.worker <- p.idle[0]
		p.idle = p.idle[1:]
		return t
	}
	p.waiting = append(p.waiting, t)
	return t
}

// Wait returns the reserved worker, waiting until one is free or ctx is done
func (t *WorkerTicket) Wait(ctx context.Context) (*OktetoBuilder, error) {
	select {
	case w := <-t.worker:
		return w, nil
	case <-ctx.Done():
		p := t.pool
		p.mu.Lock()
		if i := slices.Index(p.waiting, t); i >= 0 {
			p.waiting = slices.Delete(p.waiting, i, i+1)
			p.mu.Unlock()
			return nil, ctx.Err()
		}
		p.mu.Unlock()
		// A worker was handed to this ticket at the same time: give it back
		p.Release(<-t.worker)
		return nil, ctx.Err()
	}
}

// Acquire reserves a worker and waits for it. See Reserve.
func (p *WorkerPool) Acquire(ctx context.Context, demand int) (*OktetoBuilder, error) {
	return p.Reserve(demand).Wait(ctx)
}

// Release returns a worker to the pool once its build has finished
func (p *WorkerPool) Release(w *OktetoBuilder) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handOutLocked(w)
}

// Close cancels the extra workers that are still connecting, stops the connectors of
// every extra worker and gives the builder's connector back its output. It must be
// called once all builds have finished. It never stops the builder's own connector,
// which belongs to the caller.
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
	if p.restoreBaseOutput != nil {
		p.restoreBaseOutput()
		p.restoreBaseOutput = nil
	}
}

// handOutLocked gives w to the oldest reservation waiting for a worker, or keeps it idle
func (p *WorkerPool) handOutLocked(w *OktetoBuilder) {
	if len(p.waiting) > 0 {
		t := p.waiting[0]
		p.waiting = p.waiting[1:]
		t.worker <- w
		return
	}
	p.idle = append(p.idle, w)
}

// growLocked starts creating workers in the background until the pool has as many as
// demand, or its size
func (p *WorkerPool) growLocked(demand int) {
	for p.total < min(demand, p.size) && p.ctx.Err() == nil {
		p.total++
		p.wg.Add(1)
		go p.addExtraWorker()
	}
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

	if p.ctx.Err() != nil {
		return
	}
	if err := conn.Start(p.ctx); err != nil {
		p.base.logger.Logger().Infof("could not start an extra BuildKit connector: %s", err)
		return
	}
	p.Release(p.newWorker(conn))
}

// newWorker returns a builder that shares the base builder's context, filesystem and
// logger but uses conn and has its own metadata
func (p *WorkerPool) newWorker(conn BuildkitConnector) *OktetoBuilder {
	return NewOktetoBuilder(p.base.OktetoContext, p.base.Fs, p.base.logger, conn)
}
