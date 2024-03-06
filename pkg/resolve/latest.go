package resolve

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type FindLatestOptions struct {
	PullHost   string
	Channel    string
	HTTPClient *http.Client
}

func FindLatest(ctx context.Context, opts FindLatestOptions) (string, error) {
	opts = setFindLatestOptionsDefaults(opts)
	versionsURI := fmt.Sprintf("%s/cli/%s/versions", opts.PullHost, opts.Channel)
	resp, err := opts.HTTPClient.Get(versionsURI)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error reading versions file from %s: %s", versionsURI, resp.Status)
	}

	// @fran - 2024/02/14
	// Read at most 100mb. We need to read the whole response because the latest
	// version is last but we don't want to use io.ReadAll.
	// At the time of writing the versions file is 504 bytes and it's very unlikely
	// it will ever close to 100mb. If it does, we'll notice it in UX first
	reader := io.LimitReader(resp.Body, 100*1024*1024)

	scanner := bufio.NewScanner(reader)

	var version string
	for scanner.Scan() {
		// We need to read the whole file because the latest version is last
		version = scanner.Text()
	}

	if version == "" {
		return "", fmt.Errorf("versions file is empty")
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to find latest version: %v", err)
	}

	resp.Body.Close()

	return version, nil
}

func setFindLatestOptionsDefaults(o FindLatestOptions) FindLatestOptions {
	if o.Channel == "" {
		o.Channel = defaultChannel
	}
	if o.PullHost == "" {
		o.PullHost = defaultPullHost
	}
	if o.HTTPClient == nil {
		o.HTTPClient = &http.Client{
			Timeout:   time.Second * 10,
			Transport: defaultTransport,
		}
	}
	return o
}
