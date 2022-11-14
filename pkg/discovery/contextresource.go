// Copyright 2022 The Okteto Authors
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

package discovery

import (
	oktetoLog "github.com/okteto/okteto/pkg/log"
)

// GetContextResourcePath returns the file that will load the context resource.
// Here we only read files which have context information - no k8s or helm files
func GetContextResourcePath(wd string) (string, error) {

	oktetoManifestPath, err := GetOktetoManifestPath(wd)
	if err == nil {
		oktetoLog.Infof("context will load from %s", oktetoManifestPath)
		return oktetoManifestPath, nil
	}

	oktetoPipelineManifestPath, err := GetOktetoPipelinePath(wd)
	if err == nil {
		oktetoLog.Infof("context will load from %s", oktetoPipelineManifestPath)
		return oktetoPipelineManifestPath, nil
	}

	composeManifestPath, err := GetComposePath(wd)
	if err == nil {
		oktetoLog.Infof("context will load from %s", composeManifestPath)
		return composeManifestPath, nil
	}

	return "", ErrOktetoManifestNotFound
}
