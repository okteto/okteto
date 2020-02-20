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
	"os"

	"github.com/okteto/okteto/pkg/cmd/build"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
)

const (
	// HelmDriver default helm driver
	HelmDriver = "secrets"
)

//Translate translates the original stack based on the cluster type and built image sha256's
func Translate(s *model.Stack, forceBuild bool) error {
	c, _, _, err := k8Client.GetLocal()
	if err != nil {
		return fmt.Errorf("error creating kubernetes client: %s", err)
	}
	n, err := namespaces.Get(s.Namespace, c)
	if err == nil {
		s.Okteto = namespaces.IsOktetoNamespace(n)
	}

	for i, svc := range s.Services {
		svc.Image = os.ExpandEnv(svc.Image)
		s.Services[i] = svc
	}

	if !forceBuild {
		return nil
	}

	oktetoRegistryURL := ""
	if s.Okteto {
		oktetoRegistryURL, err = okteto.GetRegistry()
		if err != nil {
			return err
		}
	}
	buildKitHost, isOktetoCluster, err := build.GetBuildKitHost()
	if err != nil {
		return err
	}

	for name, svc := range s.Services {
		if svc.Build == "" {
			continue
		}
		log.Information("Building image for '%s'", name)
		imageTag := build.GetImageTag(name, s.Namespace, "", svc.Image, oktetoRegistryURL)
		var imageDigest string
		imageDigest, err = build.Run(buildKitHost, isOktetoCluster, svc.Build, "", imageTag, "", false)
		if err != nil {
			return fmt.Errorf("error building image for '%s': %s", name, err)
		}
		if imageDigest != "" {
			imageWithoutTag := build.GetRepoNameWithoutTag(imageTag)
			imageTag = fmt.Sprintf("%s@%s", imageWithoutTag, imageDigest)
		}
		svc.Image = imageTag
		s.Services[name] = svc
		log.Success("Image for '%s' successfully built", name)
	}

	return nil
}
