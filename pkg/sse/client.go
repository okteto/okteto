package sse

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
)

// Client represents a sse client
type Client struct {
	http http.Client
}

const (
	// pipelineSSEUrl is the template for pipeline logs sse event subscription
	pipelineSSEUrl = "%s/sse/logs/%s/gitdeploy/%s?action=%s"
)

// NewClient returns a new SSE client
func NewClient() *Client {
	return &Client{
		http: http.Client{},
	}
}

// createRequest returns an authenticated request for the url provided
func createRequest(url string) (*http.Request, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", okteto.Context().Token))
	return req, nil
}

// StreamPipelineLogs retrieves logs from the pipeline provided and prints them, returns error
func (c *Client) StreamPipelineLogs(name, actionName string) error {
	streamURL := fmt.Sprintf(pipelineSSEUrl, okteto.Context().Name, okteto.Context().Namespace, name, actionName)
	url, err := url.Parse(streamURL)
	if err != nil {
		return err
	}
	req, err := createRequest(url.String())
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	sc := bufio.NewScanner(resp.Body)

	for sc.Scan() {
		msg := getMessage(sc.Text())
		if isDone(msg) {
			return nil
		}
		if msg != "" {
			var eventLog *oktetoLog.JSONLogFormat
			if err := json.Unmarshal([]byte(msg), &eventLog); err != nil {
				continue
			}
			oktetoLog.Println(eventLog.Message)
		}
	}
	return nil
}
