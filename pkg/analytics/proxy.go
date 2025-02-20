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

const (
	unsupportedProxyContentTypeEvent = "Unsupported Proxy Content Type"
	contentTypeKey                   = "contentType"
)

// TrackUnsupportedContentType sends a tracking event to mixpanel when the proxy receives a request with an unsupported content type
func (a *Tracker) TrackUnsupportedContentType(contentType string) {
	a.trackFn(unsupportedProxyContentTypeEvent, true, map[string]interface{}{
		contentTypeKey: contentType,
	})
}
