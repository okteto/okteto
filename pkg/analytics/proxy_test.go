// Copyright 2025 The Okteto Authors
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

package analytics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProxyUnsupportedContentType(t *testing.T) {
	contentType := ""
	event := ""
	tracker := Tracker{
		trackFn: func(e string, success bool, props map[string]any) {
			event = e
			contentType = props[contentTypeKey].(string)
		},
	}
	tracker.TrackUnsupportedContentType("application/json")
	assert.Equal(t, unsupportedProxyContentTypeEvent, event)
	assert.Equal(t, "application/json", contentType)
}
