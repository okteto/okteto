package sse

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	oktetoLog "github.com/okteto/okteto/pkg/log"
)

var (
	// gitDeployUrlTemplate (baseURL, namespace, dev environment name, action name)
	GitDeployUrlTemplate = "%s/sse/logs/%s/gitdeploy/%s?action=%s"
)

const (
	maxRetryAttempts = 3
)

func readBody(ctx context.Context, body io.ReadCloser) error {
	sc := bufio.NewScanner(body)
	dataHeader := "data: "
	for sc.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			scanText := sc.Text()

			msg := ""
			// if the text scanned is a data message, trim and save into msg
			if strings.HasPrefix(scanText, dataHeader) {
				msg = strings.TrimPrefix(scanText, dataHeader)
			}
			// the msg from sse data is a string with a json log format or slice of json log format
			eventLogSlice := []oktetoLog.JSONLogFormat{}
			json.Unmarshal([]byte(msg), &eventLogSlice)
			if len(eventLogSlice) > 0 {
				for _, e := range eventLogSlice {
					if e.Message == "" {
						continue
					}
					// stop the scanner when the event log is in stage done and message is EOF
					if e.Stage == "done" && e.Message == "EOF" {
						break
					}
					oktetoLog.Println(e.Message)
				}
				continue
			}
			eventLog := &oktetoLog.JSONLogFormat{}
			// unmarshall errors ignored
			json.Unmarshal([]byte(msg), &eventLog)
			// skip when the message is empty
			if eventLog.Message == "" {
				continue
			}
			// stop the scanner when the event log is in stage done and message is EOF
			if eventLog.Stage == "done" && eventLog.Message == "EOF" {
				break
			}
			oktetoLog.Println(eventLog.Message)
		}
	}

	//return whether the scan has encountered any error
	return sc.Err()
}

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
			fmt.Println("*****errrorr request")
			continue
		}

		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}

		if resp.StatusCode != http.StatusInternalServerError {
			return nil, fmt.Errorf("error retrieving logs")
		}

		if attempts >= maxRetryAttempts {
			return nil, fmt.Errorf("server disconnected, maxRetries reached")
		}

		oktetoLog.Warning("sse client not reachable, waiting to reconnect...")
		delay := nextRetrySchedule(attempts)
		time.Sleep(delay)
	}
}

func Stream(ctx context.Context, c *http.Client, url string) error {
	resp, err := requestWithRetry(c, url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return readBody(ctx, resp.Body)
}
