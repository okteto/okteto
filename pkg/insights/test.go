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

package insights

import (
	"context"
	"encoding/json"

	"github.com/okteto/okteto/pkg/analytics"
)

const (
	// testInsightType represents the type of the test event
	testInsightType = "test"

	// testSchemaVersion represents the schema version of the test event
	// This version should be updated if the structure of the event changes
	testSchemaVersion = "1.0"
)

// testEventJSON represents the JSON structure of a test event
type testEventJSON struct {
	DevenvName    string  `json:"devenv_name"`
	Namespace     string  `json:"namespace"`
	TestName      string  `json:"test_name"`
	Repository    string  `json:"repository"`
	SchemaVersion string  `json:"schema_version"`
	Duration      float64 `json:"duration"`
	Success       bool    `json:"success"`
}

// TrackTest tracks a test execution event
func (ip *Publisher) TrackTest(ctx context.Context, meta *analytics.SingleTestMetadata) {
	eventJSON, err := json.Marshal(ip.convertTestMetadataToEvent(meta))
	if err != nil {
		ip.ioCtrl.Logger().Infof("failed to marshal event metadata: %s", err)
		return
	}

	ip.trackEvent(ctx, meta.Namespace, testInsightType, string(eventJSON))
}

// convertTestMetadataToEvent converts an SingleTestMetadata to a testEventJSON
func (*Publisher) convertTestMetadataToEvent(metadata *analytics.SingleTestMetadata) testEventJSON {
	return testEventJSON{
		DevenvName:    metadata.DevenvName,
		Namespace:     metadata.Namespace,
		TestName:      metadata.TestName,
		Repository:    metadata.Repository,
		Duration:      metadata.Duration.Seconds(),
		Success:       metadata.Success,
		SchemaVersion: testSchemaVersion,
	}
}
