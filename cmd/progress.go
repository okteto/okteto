package cmd

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cheggaaa/pb/v3"
	getter "github.com/hashicorp/go-getter"
)

var defaultProgressBar getter.ProgressTracker = &progressBar{}

type progressBar struct {
	// lock everything below
	lock sync.Mutex
}

// TrackProgress instantiates a new progress bar that will
// display the progress of stream until closed.
// total can be 0.
func (cpb *progressBar) TrackProgress(src string, currentSize, totalSize int64, stream io.ReadCloser) io.ReadCloser {
	cpb.lock.Lock()
	defer cpb.lock.Unlock()

	newPb := pb.New64(totalSize)
	newPb.Set("prefix", filepath.Base(src))
	newPb.SetCurrent(currentSize)
	//bar.Prefix(prefix)
	newPb.Start()
	reader := newPb.NewProxyReader(stream)

	return &readCloser{
		Reader: reader,
		close: func() error {
			cpb.lock.Lock()
			defer cpb.lock.Unlock()

			newPb.Finish()
			return nil
		},
	}
}

type readCloser struct {
	io.Reader
	close func() error
}

func (c *readCloser) Close() error { return c.close() }

func renderProgressBar(current int) string {
	var sb strings.Builder
	_, _ = sb.WriteString("[")

	for i := 0; i < current-1; i++ {
		_, _ = sb.WriteString("-")
	}

	sb.WriteString(">")
	for i := current + 1; i < 100; i++ {
		_, _ = sb.WriteString("_")
	}

	for i := 0; i < 100; i++ {
		if i == current {

		}
		_, _ = sb.WriteString("_")
	}
	_, _ = sb.WriteString("]")

	_, _ = sb.WriteString(" ")
	_, _ = sb.WriteString(fmt.Sprintf("%d%%", current))
	return sb.String()
}
