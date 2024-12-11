// Copyright 2024 The Okteto Authors
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

package okteto

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// pageContainsAnchor takes the HTML content and an anchor href (e.g. "#some-section"),
// and returns true if the HTML contains an anchor link with that href.
func pageContainsAnchor(htmlContent string, anchorHref string) (bool, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return false, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Traverse the HTML node tree
	var f func(*html.Node) bool
	f = func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == "a" {
			// Check the attributes of the <a> tag for the given href
			for _, attr := range n.Attr {
				if attr.Key == "href" && attr.Val == anchorHref {
					return true
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if f(c) {
				return true
			}
		}
		return false
	}

	return f(doc), nil
}

func TestJsonSchema(t *testing.T) {
	url := "https://www.okteto.com/docs/reference/okteto-manifest"
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("failed to fetch page: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	htmlContent := string(bodyBytes)
	anchorHref := "#build-object-optional"

	found, err := pageContainsAnchor(htmlContent, anchorHref)
	if err != nil {
		t.Fatalf("error while searching for anchor: %v", err)
	}

	if !found {
		t.Errorf("anchor with href '%s' not found in the page", anchorHref)
	}
}
