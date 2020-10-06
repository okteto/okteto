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
	"context"
	"fmt"
	"os"

	"github.com/okteto/okteto/pkg/cmd/build"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/subosito/gotenv"
)

const (
	stackHelmRepoURL      = "https://apps.okteto.com"
	stackHelmRepoName     = "okteto-charts"
	stackHelmChartName    = "stacks"
	stackHelmChartVersion = "0.1.0"
	helmDriver            = "secrets"
)

func translate(ctx context.Context, s *model.Stack, forceBuild, noCache bool) error {
	if err := translateEnvVars(s); err != nil {
		return nil
	}

	if forceBuild {
		if err := translateBuildImages(ctx, s, noCache); err != nil {
			return nil
		}
	}
	return nil
}

func translateEnvVars(s *model.Stack) error {
	var err error
	for name, svc := range s.Services {
		svc.Image, err = model.ExpandEnv(svc.Image)
		if err != nil {
			return err
		}
		for _, envFilepath := range svc.EnvFiles {
			translateEnvFile(&svc, envFilepath)
		}
		svc.EnvFiles = nil
		s.Services[name] = svc
	}
	return nil
}

func translateEnvFile(svc *model.Service, filename string) error {
	var err error
	filename, err = model.ExpandEnv(filename)
	if err != nil {
		return err
	}

	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	envMap, err := gotenv.StrictParse(f)
	if err != nil {
		return fmt.Errorf("error parsing env_file %s: %s", filename, err.Error())
	}
	for _, e := range svc.Environment {
		if _, ok := envMap[e.Name]; ok {
			delete(envMap, e.Name)
		}
	}
	for name, value := range envMap {
		svc.Environment = append(
			svc.Environment,
			model.EnvVar{Name: name, Value: value},
		)
	}
	return nil
}

func translateBuildImages(ctx context.Context, s *model.Stack, noCache bool) error {
	c, _, configNamespace, err := k8Client.GetLocal("")
	if err != nil {
		return err
	}
	if s.Namespace == "" {
		s.Namespace = configNamespace
	}

	oktetoRegistryURL := ""
	n, err := namespaces.Get(ctx, s.Namespace, c)
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
		imageDigest, err = build.Run(ctx, buildKitHost, isOktetoCluster, svc.Build.Context, svc.Build.Dockerfile, imageTag, svc.Build.Target, noCache, imageTag, buildArgs, "tty")
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
