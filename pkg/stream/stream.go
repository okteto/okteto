// Copyright 2022 The Okteto Authors
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

package stream

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	oktetoLog "github.com/okteto/okteto/pkg/log"
)

const (
	maxRetryAttempts = 3
	dataPing         = "ping"
	dataHeader       = "data: "
)

func nextRetrySchedule(attempts int) time.Duration {
	delaySecs := int64(math.Floor((math.Pow(2, float64(attempts)) - 1) * 0.5))
	return time.Duration(delaySecs) * time.Second
}

func request(c *http.Client, url string) (*http.Response, error) {
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func requestWithRetry(c *http.Client, url string) (*http.Response, error) {
	attempts := 0
	for {
		attempts++
		resp, err := request(c, url)
		if err != nil {
			continue
		}

		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}

		if resp.StatusCode != http.StatusInternalServerError {
			return nil, fmt.Errorf("response from request: %s", resp.Status)
		}

		if attempts >= maxRetryAttempts {
			return nil, fmt.Errorf("server disconnected, maxRetries reached")
		}

		oktetoLog.Warning("stream client not reachable, waiting to reconnect...")
		delay := nextRetrySchedule(attempts)
		time.Sleep(delay)
	}
}

// printFn represents the function that prints the log message, returns true when is "done" message
type printFn func(line string) bool

func GetLogsFromURL(ctx context.Context, c *http.Client, url string, print printFn) error {
	resp, err := requestWithRetry(c, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	sc := bufio.NewScanner(resp.Body)
	done := false
	for sc.Scan() && !done {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			scanText := sc.Text()
			// if the text scanned is a data message, trim and print
			if strings.HasPrefix(scanText, dataHeader) {
				data := strings.TrimSpace(strings.TrimPrefix(scanText, dataHeader))
				if data == dataPing {
					continue
				}
				done = print(data)
			}
		}
	}

	// return whether the scan has encountered any error
	return sc.Err()
}
