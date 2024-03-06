package resolve

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/cavaliergopher/grab/v3"
)

type PullOptions struct {
	Channel          string
	PullHost         string
	BinDirectory     string
	ProgressInterval time.Duration
	HTTPClient       *http.Client
}

type DownloadProgress struct {
	Completed int64
	Size      int64
	Progress  float64

	Done        bool
	Error       error
	Destination string
}

func Pull(ctx context.Context, version string, opts PullOptions) (<-chan DownloadProgress, error) {
	opts = setOptionsDefaults(opts)
	pullURI := fmt.Sprintf("%s/cli/%s/%s/%s", opts.PullHost, opts.Channel, version, binFilename)
	pchan := make(chan DownloadProgress)
	dest := path.Join(opts.BinDirectory, fmt.Sprintf("%s/okteto", version))

	req, err := grab.NewRequest(dest, pullURI)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	go func() {

		c := grab.NewClient()
		c.UserAgent = "okteto"
		c.HTTPClient = opts.HTTPClient
		resp := c.Do(req)
		t := time.NewTicker(opts.ProgressInterval)
		defer t.Stop()
		defer close(pchan)
		for {
			select {
			case <-t.C:
				pchan <- DownloadProgress{
					Completed:   resp.BytesComplete(),
					Size:        resp.Size(),
					Progress:    resp.Progress(),
					Destination: resp.Filename,
					Done:        false,
				}
			case <-resp.Done:
				e := resp.Err()
				if e == nil {
					e = chmodToExec(resp.Filename)
				}
				pchan <- DownloadProgress{
					Completed:   resp.BytesComplete(),
					Size:        resp.Size(),
					Progress:    resp.Progress(),
					Done:        true,
					Error:       e,
					Destination: resp.Filename,
				}
				return
			}
		}
	}()

	return pchan, err
}

func setOptionsDefaults(o PullOptions) PullOptions {
	if o.BinDirectory == "" {
		o.BinDirectory = versionedBinDir()
	}
	if o.Channel == "" {
		o.Channel = defaultChannel
	}
	if o.PullHost == "" {
		o.PullHost = defaultPullHost
	}
	if o.HTTPClient == nil {
		o.HTTPClient = &http.Client{
			Timeout:   time.Minute * 2, // it might take a while to download the binary
			Transport: defaultTransport,
		}
	}
	if o.ProgressInterval == 0 {
		o.ProgressInterval = 500 * time.Millisecond
	}
	return o
}

// chmodToExec adds executable permissions to a file preserving read original
// read and write permissions
func chmodToExec(filename string) error {
	fileInfo, err := os.Stat(filename)
	if err != nil {
		return err
	}

	newMode := fileInfo.Mode() | 0111
	return os.Chmod(filename, newMode)
}
