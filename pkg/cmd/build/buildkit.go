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

package build

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/okteto/okteto/pkg/buildkit"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
)

const (
	frontend = "dockerfile.v0"
)

//GetBuildKitHost returns thee buildkit url
func GetBuildKitHost() (string, bool, error) {
	buildKitHost := os.Getenv("BUILDKIT_HOST")
	//TODO dont support this use case
	if buildKitHost != "" {
		log.Information("Running your build in %s...", buildKitHost)
		return buildKitHost, false, nil
	}
	buildkitURL, err := okteto.GetBuildKit()
	if err != nil {
		return "", false, err
	}
	//TODO print info out of this function
	if buildkitURL == okteto.CloudBuildKitURL {
		log.Information("Running your build in Okteto Cloud...")
	} else {
		log.Information("Running your build in Okteto Enterprise...")
	}
	return buildkitURL, true, err
}

//GetSolveOpt returns the buildkit solve options
func GetSolveOpt(buildCtx, file, imageTag, target string, noCache bool) (*client.SolveOpt, error) {
	if file == "" {
		file = filepath.Join(buildCtx, "Dockerfile")
	}
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return nil, fmt.Errorf("Dockerfile '%s' does not exist", file)
	}
	localDirs := map[string]string{
		"context":    buildCtx,
		"dockerfile": filepath.Dir(file),
	}

	frontendAttrs := map[string]string{
		"filename": filepath.Base(file),
	}
	if target != "" {
		frontendAttrs["target"] = target
	}
	if noCache {
		frontendAttrs["no-cache"] = ""
	}

	attachable := []session.Attachable{}
	token, err := okteto.GetToken()
	if err == nil {
		registryURL, err := okteto.GetRegistry()
		if err != nil {
			return nil, err
		}
		attachable = append(attachable, buildkit.NewDockerAndOktetoAuthProvider(registryURL, okteto.GetUserID(), token.Token, os.Stderr))
	} else {
		attachable = append(attachable, authprovider.NewDockerAuthProvider(os.Stderr))
	}
	opt := &client.SolveOpt{
		LocalDirs:     localDirs,
		Frontend:      frontend,
		FrontendAttrs: frontendAttrs,
		Session:       attachable,
	}

	if imageTag != "" {
		opt.Exports = []client.ExportEntry{
			{
				Type: "image",
				Attrs: map[string]string{
					"name": imageTag,
					"push": "true",
				},
			},
		}
		opt.CacheExports = []client.CacheOptionsEntry{
			{
				Type: "inline",
			},
		}
		opt.CacheImports = []client.CacheOptionsEntry{
			{
				Type:  "registry",
				Attrs: map[string]string{"ref": imageTag},
			},
		}
	}

	return opt, nil
}
