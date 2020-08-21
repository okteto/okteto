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

package stack

import (
	"fmt"
	"os"

	"github.com/okteto/okteto/pkg/cmd/build"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
)

const (
	stackHelmRepoURL      = "https://apps.okteto.com"
	stackHelmRepoName     = "okteto-charts"
	stackHelmChartName    = "stacks"
	stackHelmChartVersion = "0.1.0"
	helmDriver            = "secrets"
)

func translate(s *model.Stack, forceBuild, noCache bool) error {
	for i, svc := range s.Services {
		svc.Image = os.ExpandEnv(svc.Image)
		s.Services[i] = svc
	}

	if !forceBuild {
		return nil
	}

	c, _, configNamespace, err := k8Client.GetLocal("")
	if err != nil {
		return err
	}
	if s.Namespace == "" {
		s.Namespace = configNamespace
	}

	oktetoRegistryURL := ""
	n, err := namespaces.Get(s.Namespace, c)
	if err == nil {
		if namespaces.IsOktetoNamespace(n) {
			oktetoRegistryURL, err = okteto.GetRegistry()
			if err != nil {
				return err
			}
		}
	}

	buildKitHost, isOktetoCluster, err := build.GetBuildKitHost()
	if err != nil {
		return err
	}

	oneBuild := false
	for name, svc := range s.Services {
		if svc.Build == nil {
			continue
		}
		oneBuild = true
		imageTag := build.GetImageTag(svc.Image, name, s.Namespace, oktetoRegistryURL)
		log.Information("Building image for service '%s'...", name)
		var imageDigest string
		buildArgs := model.SerializeBuildArgs(svc.Build.Args)
		imageDigest, err = build.Run(buildKitHost, isOktetoCluster, svc.Build.Context, svc.Build.Dockerfile, imageTag, svc.Build.Target, noCache, imageTag, buildArgs, "tty")
		if err != nil {
			return fmt.Errorf("error building image for '%s': %s", name, err)
		}
		if imageDigest != "" {
			imageWithoutTag := build.GetRepoNameWithoutTag(imageTag)
			imageTag = fmt.Sprintf("%s@%s", imageWithoutTag, imageDigest)
		}
		svc.Image = imageTag
		s.Services[name] = svc
		log.Success("Image for service '%s' successfully pushed", name)
	}

	if !oneBuild {
		log.Information("No build directives found in your Stack manifest")
	}

	return nil
}
