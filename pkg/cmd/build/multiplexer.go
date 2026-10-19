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
	"bytes"
	"context"
	"fmt"
	stdio "io"
	"os"
	"strings"
	"sync"

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/moby/buildkit/util/progress/progresswriter"
	"github.com/okteto/okteto/pkg/build/buildkit"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
)

// Multiplexer shows the progress of several builds running at the same time as one
// output, the way docker buildx bake does. In tty mode every build feeds one live display
// where its steps are prefixed with its service name. In any other mode each build prints
// plain lines prefixed with its service name and tagged with its own stage. In both, the
// messages printed by the builds (pushed images, warnings) are held and shown together at
// the end.
type Multiplexer struct {
	out      *os.File
	printer  progresswriter.Writer
	pending  *lockedWriter
	services map[string]*muxService
	progress string
}

type muxService struct {
	// ioCtrl prints the messages of the builds (pushed images, warnings), held until Close
	ioCtrl *io.Controller
	// progressIO prints the plain progress lines as they come; nil in tty mode
	progressIO *io.Controller
	// writer feeds the shared tty display; nil in plain mode
	writer   progresswriter.Writer
	name     string
	tty      bool
	mu       sync.Mutex
	finished bool
}

// lockedWriter makes every Write atomic, so lines of concurrent builds never mix
type lockedWriter struct {
	w  stdio.Writer
	mu sync.Mutex
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

// NewMultiplexer prepares the output for building services concurrently. progress is
// resolved like the progress mode of each build (see OptsFromBuildInfo). Every service
// that may build must be listed up front, before any build starts: the shared display
// ends once all of them are finished.
func NewMultiplexer(ioCtrl *io.Controller, progress string, services []string) (*Multiplexer, error) {
	return newMultiplexer(ioCtrl, buildOutputMode(progress), services, os.Stdout, progressui.TtyMode)
}

func newMultiplexer(ioCtrl *io.Controller, progress string, services []string, out *os.File, displayMode progressui.DisplayMode) (*Multiplexer, error) {
	m := &Multiplexer{
		out:      out,
		progress: progress,
		services: make(map[string]*muxService, len(services)),
		pending:  &lockedWriter{w: &bytes.Buffer{}},
	}
	tty := progress == oktetoLog.TTYFormat && len(services) > 0
	var mw *progresswriter.MultiWriter
	progressSink := &lockedWriter{w: out}
	if tty {
		ioCtrl.Out().Println(fmt.Sprintf("Building %d services in parallel: %s", len(services), strings.Join(services, ", ")))
		printer, err := newPrinter(out, displayMode)
		if err != nil {
			return nil, err
		}
		m.printer = printer
		mw = progresswriter.NewMultiWriter(printer)
	}
	for _, svc := range services {
		stage := fmt.Sprintf("Building service %s", svc)
		s := &muxService{
			name:   svc,
			tty:    tty,
			ioCtrl: ioCtrl.WithOutput(m.pending, stage),
		}
		if tty {
			s.writer = mw.WithPrefix(svc, true)
		} else {
			s.progressIO = ioCtrl.WithOutput(progressSink, stage)
		}
		m.services[svc] = s
	}
	return m, nil
}

// newPrinter returns the shared display, falling back to plain mode when the terminal
// doesn't support the requested mode
func newPrinter(out *os.File, mode progressui.DisplayMode) (progresswriter.Writer, error) {
	// not using shared context to not disrupt display but let it finish reporting errors
	printer, err := progresswriter.NewPrinter(context.TODO(), out, string(mode))
	if err != nil {
		return progresswriter.NewPrinter(context.TODO(), out, string(progressui.PlainMode))
	}
	return printer, nil
}

// SolveFn returns the solve function for the builds of svc
func (m *Multiplexer) SolveFn(svc string) buildkit.SolveBuildFn {
	s := m.services[svc]
	return func(ctx context.Context, c *client.Client, opt *client.SolveOpt, _ string, _ *io.Controller, metadata *buildkit.BuildMetadata) error {
		return solve(ctx, c, opt, m.progress, metadata, s.display)
	}
}

// IO returns the io controller for the builds of svc
func (m *Multiplexer) IO(svc string) *io.Controller {
	return m.services[svc].ioCtrl
}

// Finish marks the build of svc as over, including any retry. Its steps stop being
// shown in the shared display.
func (m *Multiplexer) Finish(svc string) {
	m.services[svc].finish()
}

// Close finishes every service, waits for the shared display to end and then prints the
// messages the builds wrote while they were running. Call it once all builds are over.
func (m *Multiplexer) Close() error {
	for _, s := range m.services {
		s.finish()
	}
	var displayErr error
	if m.printer != nil {
		<-m.printer.Done()
		displayErr = m.printer.Err()
	}
	if _, err := m.out.Write(m.pending.w.(*bytes.Buffer).Bytes()); err != nil {
		return err
	}
	return displayErr
}

// display shows the status of one solve of the service
func (s *muxService) display(ch chan *client.SolveStatus, _ chan<- error) error {
	if !s.tty {
		return s.showPlain(ch)
	}
	buffer := make(chan *client.SolveStatus)
	bufferDone := make(chan struct{})
	go func() {
		defer close(bufferDone)
		w := &prefixWriter{prefix: fmt.Sprintf("[%s] ", s.name), out: &buildWriter{}}
		showInLogBuffer(buffer, w)
		if err := w.flush(); err != nil {
			oktetoLog.Infof("could not write build status to the log buffer: %s", err)
		}
	}()
	for ss := range ch {
		buffer <- ss
		s.send(copyStatus(ss))
	}
	close(buffer)
	<-bufferDone
	return nil
}

// showPlain prints the status as plain lines prefixed with the service name
func (s *muxService) showPlain(ch chan *client.SolveStatus) error {
	w := &prefixWriter{prefix: fmt.Sprintf("[%s] ", s.name), out: s.progressIO.Out()}
	d, err := progressui.NewDisplay(w, progressui.PlainMode)
	if err != nil {
		oktetoLog.Infof("could not display build status: %s", err)
		return err
	}
	// not using shared context to not disrupt display but let it finish reporting errors
	if _, err := d.UpdateFrom(context.TODO(), ch); err != nil {
		oktetoLog.Infof("could not display build status: %s", err)
	}
	return w.flush()
}

// send forwards a status to the shared display unless the service is finished
func (s *muxService) send(ss *client.SolveStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return
	}
	s.writer.Status() <- ss
}

func (s *muxService) finish() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished {
		return
	}
	s.finished = true
	if s.writer != nil {
		close(s.writer.Status())
	}
}

// copyStatus copies the vertexes of a status, so the shared display can rename them
// while the log buffer display reads the original
func copyStatus(ss *client.SolveStatus) *client.SolveStatus {
	cp := *ss
	cp.Vertexes = make([]*client.Vertex, len(ss.Vertexes))
	for i, v := range ss.Vertexes {
		vertex := *v
		cp.Vertexes[i] = &vertex
	}
	return &cp
}

// prefixWriter writes every complete line it receives prefixed, skipping empty lines.
// BuildKit's plain display may write a line in several calls (text, then its new line),
// so incomplete lines are kept until they end or flush is called.
type prefixWriter struct {
	out     stdio.Writer
	prefix  string
	pending []byte
}

func (p *prefixWriter) Write(b []byte) (int, error) {
	p.pending = append(p.pending, b...)
	for {
		i := bytes.IndexByte(p.pending, '\n')
		if i < 0 {
			return len(b), nil
		}
		line := p.pending[:i]
		p.pending = p.pending[i+1:]
		if err := p.writeLine(line); err != nil {
			return 0, err
		}
	}
}

// flush writes the incomplete line, if any
func (p *prefixWriter) flush() error {
	line := p.pending
	p.pending = nil
	return p.writeLine(line)
}

func (p *prefixWriter) writeLine(line []byte) error {
	if len(bytes.TrimSpace(line)) == 0 {
		return nil
	}
	_, err := p.out.Write([]byte(p.prefix + string(line) + "\n"))
	return err
}
