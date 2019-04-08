package syncthing

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"time"
)

type addAPIKeyTransport struct {
	T http.RoundTripper
}

// API types

type syncthingConnections struct {
	Connections map[string]struct {
		Connected bool `json:"connected,omitempty"`
	} `json:"connections,omitempty"`
}

type event struct {
	Type string `json:"type,omitempty"`
	Data struct {
		Completion  float64 `json:"completion,omitempty"`
		Folder      string  `json:"folder,omitempty"`
		Device      string  `json:"device,omitempty"`
		GlobalBytes int64   `json:"globalBytes,omitempty"`
	} `json:"data,omitempty"`
}

type syncthingErrors struct {
	Errors []struct {
		Message string `json:"message,omitempty"`
	} `json:"errors,omitempty"`
}

type completion struct {
	Completion float64 `json:"completion,omitempty"`
	NeedBytes  int     `json:"needBytes,omitempty"`
}

func (akt *addAPIKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("X-API-Key", "cnd")
	return akt.T.RoundTrip(req)
}

//NewAPIClient returns a new syncthing api client configured to call the syncthing api
func NewAPIClient() *http.Client {
	return &http.Client{
		Timeout:   5 * time.Second,
		Transport: &addAPIKeyTransport{http.DefaultTransport},
	}
}

// APICall calls the syncthing API and returns the parsed json or an error
func (s *Syncthing) APICall(url, method string, code int, params map[string]string, local bool) ([]byte, error) {
	var urlPath string
	if local {
		urlPath = filepath.Join(s.GUIAddress, url)
	} else {
		urlPath = filepath.Join(s.RemoteGUIAddress, url)
	}
	req, err := http.NewRequest(method, fmt.Sprintf("http://%s", urlPath), nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("limit", "30")
	if params != nil {
		for key, value := range params {
			q.Add(key, value)
		}
	}
	req.URL.RawQuery = q.Encode()

	resp, err := s.Client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != code {
		return nil, fmt.Errorf("bad response from syncthing api %s %d: %s", req.URL.String(), resp.StatusCode, string(body))
	}

	return body, nil
}
