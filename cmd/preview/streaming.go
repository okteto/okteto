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

package preview

import (
	"context"
	"errors"
	"math"
	"time"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/types"
)

// streamFn represents a function that attempts to stream logs and returns an error
type streamFn func(ctx context.Context) error

const (
	maxAttemptPower = 10
)

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

// streamWithExponentialBackoff attempts to stream logs with exponential backoff until the context timeout
func streamWithExponentialBackoff(ctx context.Context, timeout time.Duration, streamFunc streamFn, operationName string) error {
	// Create a context with the user's timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	attempt := 0

	for {
		select {
		case <-timeoutCtx.Done():
			// Timeout reached, stop retrying
			if errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
				oktetoLog.Infof("%s streaming stopped after timeout %v", operationName, timeout)
				return context.DeadlineExceeded
			}
			return timeoutCtx.Err()
		default:
			// Attempt to stream logs
			err := streamFunc(timeoutCtx)
			if err == nil {
				// Streaming completed successfully
				return nil
			}

			// Check if context was cancelled or timed out
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}

			// Calculate exponential backoff delay
			attempt++
			retryDelay := calculateExponentialBackoff(attempt)

			// Log the retry attempt
			oktetoLog.Warning("preview stream client not reachable, waiting to reconnect...")
			oktetoLog.Infof("%s streaming error: %v, retrying in %v (attempt %d)", operationName, err, retryDelay, attempt)

			// Wait before retrying, but respect context cancellation
			select {
			case <-timeoutCtx.Done():
				return timeoutCtx.Err()
			case <-time.After(retryDelay):
				// Continue to next attempt
			}
		}
	}
}

// streamPreviewLogsWithTimeout attempts to stream preview logs with exponential backoff until timeout
func streamPreviewLogsWithTimeout(ctx context.Context, okClient types.OktetoInterface, name, namespace, actionName string, timeout time.Duration) error {
	return streamWithExponentialBackoff(ctx, timeout, func(ctx context.Context) error {
		return okClient.Stream().PipelineLogs(ctx, name, namespace, actionName)
	}, "preview logs")
}

// streamDestroyLogsWithTimeout attempts to stream destroy logs with exponential backoff until timeout
func streamDestroyLogsWithTimeout(ctx context.Context, okClient types.OktetoInterface, namespace string, timeout time.Duration) error {
	return streamWithExponentialBackoff(ctx, timeout, func(ctx context.Context) error {
		return okClient.Stream().DestroyAllLogs(ctx, namespace)
	}, "destroy logs")
}
