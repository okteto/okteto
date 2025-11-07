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
package smartbuild

import (
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/registry"
)

type DigestResolver interface {
	GetImageTagWithDigest(string) (string, error)
}

type Logger interface {
	Infof(format string, args ...interface{})
}

type RegistryCacheProbe struct {
	tagger      ImageTagger
	namespace   string
	registryURL string
	imageCtrl   registry.ImageCtrl
	registry    DigestResolver
	logger      Logger
}

func NewRegistryCacheProbe(tagger ImageTagger, namespace string, registryURL string, imageCtrl registry.ImageCtrl, registry DigestResolver, logger Logger) *RegistryCacheProbe {
	return &RegistryCacheProbe{
		tagger:      tagger,
		namespace:   namespace,
		registryURL: registryURL,
		imageCtrl:   imageCtrl,
		registry:    registry,
		logger:      logger,
	}
}

func (c *RegistryCacheProbe) IsCached(manifestName, image, buildHash, svcToBuild string) (bool, string, error) {
	if buildHash == "" {
		return false, "", nil
	}

	var referencesToCheck []string
	if image != "" {
		globalImage := c.tagger.GetGlobalTagFromDevIfNeccesary(image, c.namespace, c.registryURL, buildHash, c.imageCtrl)
		if globalImage != "" {
			referencesToCheck = []string{globalImage}
		}
	} else {
		// [name]:[tag] being the tag the buildHash
		referencesToCheck = c.tagger.GetImageReferencesForTag(manifestName, svcToBuild, buildHash)
	}

	for _, ref := range referencesToCheck {
		imageWithDigest, err := c.registry.GetImageTagWithDigest(ref)
		if err != nil {
			if oktetoErrors.IsNotFound(err) {
				continue
			}
			c.logger.Infof("could not check image %s: %s", ref, err)
			// If trying to get access to the image, it fails unexpectedly, we try with any other image (if any)
			continue
		}
		c.logger.Infof("image %s found", ref)
		return true, imageWithDigest, nil
	}
	return false, "", nil
}

func (c *RegistryCacheProbe) LookupReferenceWithDigest(reference string) (string, error) {
	return c.registry.GetImageTagWithDigest(reference)
}
