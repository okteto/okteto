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

package insights

import (
	"context"
	"encoding/json"

	"github.com/okteto/okteto/pkg/analytics"
)

// buildEventJSON represents the JSON structure of a build event
type buildEventJSON struct {
	DevenvName    string  `json:"devenv_name"`
	ImageName     string  `json:"image_name"`
	Namespace     string  `json:"namespace"`
	Repository    string  `json:"repository"`
	SchemaVersion string  `json:"schema_version"`
	Duration      float64 `json:"duration"`
	SmartBuildHit bool    `json:"smart_build_hit"`
	Success       bool    `json:"success"`
}

// TrackImageBuild tracks an image build event
func (ip *Publisher) TrackImageBuild(ctx context.Context, meta *analytics.ImageBuildMetadata) {
	eventJSON, err := json.Marshal(ip.convertImageBuildMetadataToEvent(meta))
	if err != nil {
		ip.ioCtrl.Logger().Infof("failed to marshal event metadata: %s", err)
		return
	}

	ip.trackEvent(ctx, meta.Namespace, "build", string(eventJSON))
}

// convertImageBuildMetadataToEvent converts an ImageBuildMetadata to a buildEventJSON
func (*Publisher) convertImageBuildMetadataToEvent(metadata *analytics.ImageBuildMetadata) buildEventJSON {
	return buildEventJSON{
		DevenvName:    metadata.DevenvName,
		ImageName:     metadata.Name,
		Namespace:     metadata.Namespace,
		Repository:    metadata.RepoURL,
		SmartBuildHit: metadata.CacheHit,
		Success:       metadata.Success,
		Duration:      metadata.BuildDuration.Seconds(),
		SchemaVersion: "1.0",
	}
}
