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

package stream

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	oktetoLog "github.com/okteto/okteto/pkg/log"
)

const (
	dataPing        = "ping"
	dataHeader      = "data: "
	maxAttemptPower = 10
)

func request(c *http.Client, url string) (*http.Response, error) {
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func requestWithRetry(ctx context.Context, c *http.Client, url string, timeout time.Duration) (*http.Response, error) {
	attempts := 0
	for {
		select {
		case <-ctx.Done():
			// Timeout reached, stop retrying
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				oktetoLog.Infof("logs streaming stopped after timeout %v", timeout)
				return nil, context.DeadlineExceeded
			}
			return nil, ctx.Err()

		default:
			// Attempt to stream logs
			resp, err := request(c, url)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return nil, err
				}
				continue
			}
			if resp.StatusCode == http.StatusOK {
				return resp, nil
			}

			if resp.StatusCode != http.StatusInternalServerError {
				return nil, fmt.Errorf("response from request: %s", resp.Status)
			}
			// Calculate exponential backoff delay
			attempts++
			retryDelay := calculateExponentialBackoff(attempts)

			// Create a proper error message for logging
			streamError := fmt.Errorf("received status %d, retrying", resp.StatusCode)

			// Log the retry attempt
			oktetoLog.Warning("Unable to connect to stream logs, waiting to reconnect...")
			oktetoLog.Infof("logs streaming error: %v, retrying in %v (attempt %d)", streamError, retryDelay, attempts)

			// Wait before retrying, but respect context cancellation
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryDelay):
				// Continue to next attempt
			}
		}
	}
}

// handleLineFn represents the function that prints the log message, returns true when is "done" message
type handleLineFn func(line string) bool

// GetLogsFromURL makes a request to the url provided and reads the content of the body
// the client will try to retry connection if fails
// the handler will handle the content of the streaming events coming from the request body
func GetLogsFromURL(ctx context.Context, c *http.Client, url string, handler handleLineFn, timeout time.Duration) error {
	resp, err := requestWithRetry(ctx, c, url, timeout)
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
				done = handler(data)
			}
		}
	}

	// return whether the scan has encountered any error
	return sc.Err()
}

// calculateExponentialBackoff calculates the next retry delay using exponential backoff
func calculateExponentialBackoff(attempt int) time.Duration {
	// Prevent overflow for very large attempts by capping at a reasonable value
	// After attempt 6, we hit the 30s cap anyway, so no need to calculate large powers
	if attempt > maxAttemptPower {
		attempt = maxAttemptPower
	}

	baseDelaySeconds := (math.Pow(2, float64(attempt)) - 1) * 0.5
	baseDelay := time.Duration(baseDelaySeconds * float64(time.Second))

	// Cap the maximum delay at 30 seconds
	maxDelay := 30 * time.Second
	if baseDelay > maxDelay {
		baseDelay = maxDelay
	}

	// Ensure minimum delay of 500ms
	minDelay := 500 * time.Millisecond
	if baseDelay < minDelay {
		baseDelay = minDelay
	}

	return baseDelay
}
