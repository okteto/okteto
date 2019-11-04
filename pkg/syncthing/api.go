package syncthing

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"time"
)

type addAPIKeyTransport struct {
	T http.RoundTripper
}

func (akt *addAPIKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("X-API-Key", "cnd")
	return akt.T.RoundTrip(req)
}

//NewAPIClient returns a new syncthing api client configured to call the syncthing api
func NewAPIClient() *http.Client {
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: &addAPIKeyTransport{http.DefaultTransport},
	}
}

// APICall calls the syncthing API and returns the parsed json or an error
func (s *Syncthing) APICall(ctx context.Context, url, method string, code int, params map[string]string, local bool, body []byte) ([]byte, error) {
	var urlPath string
	if local {
		urlPath = path.Join(s.GUIAddress, url)
	} else {
		urlPath = path.Join(s.RemoteGUIAddress, url)
	}
	req, err := http.NewRequest(method, fmt.Sprintf("http://%s", urlPath), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)

	q := req.URL.Query()
	q.Add("limit", "30")

	for key, value := range params {
		q.Add(key, value)
	}

	req.URL.RawQuery = q.Encode()

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != code {
		return nil, fmt.Errorf("bad response from syncthing api %s %d: %s", req.URL.String(), resp.StatusCode, string(body))
	}

	return body, nil
}
