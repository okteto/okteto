// Copyright 2020 The Okteto Authors
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

package helm

import (
	"fmt"
	"path/filepath"

	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/helmpath"
	"helm.sh/helm/v3/pkg/repo"
)

//RepoUpdate updates a local helm repo
func RepoUpdate(re *repo.Entry, settings *cli.EnvSettings, repoName, chartName, chartVersion string) error {
	f := filepath.Join(settings.RepositoryCache, helmpath.CacheIndexFile(repoName))
	ind, err := repo.LoadIndexFile(f)
	if err == nil {
		chartVersions, ok := ind.Entries[chartName]
		if ok {
			for _, cv := range chartVersions {
				if cv.Version == chartVersion {
					return nil
				}
			}
		}
	}

	r, err := repo.NewChartRepository(re, getter.All(settings))
	if err != nil {
		return fmt.Errorf("error updating versions from the %s chart repository: %s", chartName, err)
	}
	if _, err := r.DownloadIndexFile(); err != nil {
		return fmt.Errorf("unable to get an update from the %s chart repository: %s", chartName, err)
	}
	return nil
}
