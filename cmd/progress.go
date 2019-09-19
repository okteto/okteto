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

func renderProgressBar(prefix string, current float64, scalingFactor float64) string {
	var sb strings.Builder
	_, _ = sb.WriteString(prefix)
	_, _ = sb.WriteString("[")

	scaledMax := int(100 * scalingFactor)
	scaledCurrent := int(current * scalingFactor)

	if scaledCurrent == 0 {
		sb.WriteString(strings.Repeat("_", scaledMax))
	} else if scaledCurrent >= scaledMax {
		sb.WriteString(strings.Repeat("-", scaledMax))
	} else {
		sb.WriteString(strings.Repeat("-", scaledCurrent-1))
		sb.WriteString(">")
		sb.WriteString(strings.Repeat("_", scaledMax-scaledCurrent))
	}

	_, _ = sb.WriteString("]")
	_, _ = sb.WriteString(fmt.Sprintf(" %3v%%", int(current)))
	return sb.String()
}
