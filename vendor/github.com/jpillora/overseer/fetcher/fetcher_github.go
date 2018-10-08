package fetcher

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime"
	"strings"
	"time"
)

//Github uses the Github V3 API to retrieve the latest release
//of a given repository and enumerate its assets. If a release
//contains a matching asset, it will fetch
//and return its io.Reader stream.
type Github struct {
	//Github username and repository name
	User, Repo string
	//Interval between fetches
	Interval time.Duration
	//Asset is used to find matching release asset.
	//By default a file will match if it contains
	//both GOOS and GOARCH.
	Asset func(filename string) bool
	//internal state
	releaseURL    string
	delay         bool
	lastETag      string
	latestRelease struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
}

func (h *Github) defaultAsset(filename string) bool {
	return strings.Contains(filename, runtime.GOOS) && strings.Contains(filename, runtime.GOARCH)
}

// Init validates the provided config
func (h *Github) Init() error {
	//apply defaults
	if h.User == "" {
		return fmt.Errorf("User required")
	}
	if h.Repo == "" {
		return fmt.Errorf("Repo required")
	}
	if h.Asset == nil {
		h.Asset = h.defaultAsset
	}
	h.releaseURL = "https://api.github.com/repos/" + h.User + "/" + h.Repo + "/releases/latest"
	if h.Interval == 0 {
		h.Interval = 5 * time.Minute
	} else if h.Interval < 1*time.Minute {
		log.Printf("[overseer.github] warning: intervals less than 1 minute will surpass the public rate limit")
	}
	return nil
}

// Fetch the binary from the provided Repository
func (h *Github) Fetch() (io.Reader, error) {
	//delay fetches after first
	if h.delay {
		time.Sleep(h.Interval)
	}
	h.delay = true
	//check release status
	resp, err := http.Get(h.releaseURL)
	if err != nil {
		return nil, fmt.Errorf("release info request failed (%s)", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("release info request failed (status code %d)", resp.StatusCode)
	}
	//clear assets
	h.latestRelease.Assets = nil
	if err := json.NewDecoder(resp.Body).Decode(&h.latestRelease); err != nil {
		return nil, fmt.Errorf("invalid request info (%s)", err)
	}
	resp.Body.Close()
	//find appropriate asset
	assetURL := ""
	for _, a := range h.latestRelease.Assets {
		if h.Asset(a.Name) {
			assetURL = a.URL
			break
		}
	}
	if assetURL == "" {
		return nil, fmt.Errorf("no matching assets in this release (%s)", h.latestRelease.TagName)
	}
	//fetch location
	req, _ := http.NewRequest("HEAD", assetURL, nil)
	resp, err = http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("release location request failed (%s)", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		return nil, fmt.Errorf("release location request failed (status code %d)", resp.StatusCode)
	}
	s3URL := resp.Header.Get("Location")
	//pseudo-HEAD request
	req, err = http.NewRequest("GET", s3URL, nil)
	if err != nil {
		return nil, fmt.Errorf("release location url error (%s)", err)
	}
	req.Header.Set("Range", "bytes=0-0") // HEAD not allowed so we request for 1 byte
	resp, err = http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("release location request failed (%s)", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("release location request failed (status code %d)", resp.StatusCode)
	}
	etag := resp.Header.Get("ETag")
	if etag != "" && h.lastETag == etag {
		return nil, nil //skip, hash match
	}
	//get binary request
	resp, err = http.Get(s3URL)
	if err != nil {
		return nil, fmt.Errorf("release binary request failed (%s)", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("release binary request failed (status code %d)", resp.StatusCode)
	}
	h.lastETag = etag
	//success!
	//extract gz files
	if strings.HasSuffix(assetURL, ".gz") && resp.Header.Get("Content-Encoding") != "gzip" {
		return gzip.NewReader(resp.Body)
	}
	return resp.Body, nil
}
