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
package checker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/build"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log/io"
)

type CacheProbe interface {
	IsCached(ctx context.Context, manifestName, image, buildHash, svcToBuild string) (bool, string, error)
	LookupReferenceWithDigest(reference string) (string, error)
	GetFromCache(svc string) (hit bool, reference string)
}

type ServiceEnvVarsSetter interface {
	SetServiceEnvVars(service, reference string)
}

type outInformer interface {
	Infof(format string, args ...interface{})
}

type SvcMetadataGetter interface {
	GetMetadata(svcName string) *analytics.ImageBuildMetadata
}

type SequentialCheckStrategy struct {
	smartBuildCtrl       SmartBuildController
	tagger               ImageTagger
	imageCacheChecker    CacheProbe
	metadataCollector    SvcMetadataGetter
	ioCtrl               outInformer
	serviceEnvVarsSetter ServiceEnvVarsSetter
}

func newSequentialCheckStrategy(smartBuildCtrl SmartBuildController, tagger ImageTagger, imageCacheChecker CacheProbe, metadataCollector SvcMetadataGetter, ioCtrl *io.Controller, serviceEnvVarsSetter ServiceEnvVarsSetter) *SequentialCheckStrategy {
	return &SequentialCheckStrategy{
		smartBuildCtrl:       smartBuildCtrl,
		tagger:               tagger,
		imageCacheChecker:    imageCacheChecker,
		metadataCollector:    metadataCollector,
		ioCtrl:               ioCtrl.Out(),
		serviceEnvVarsSetter: serviceEnvVarsSetter,
	}
}

func (s *SequentialCheckStrategy) CheckServicesCache(ctx context.Context, manifestName string, buildManifest build.ManifestBuild, svcsToBuild []string) (cachedSvcs []string, notCachedSvcs []string, err error) {
	for _, svc := range svcsToBuild {
		meta := s.metadataCollector.GetMetadata(svc)
		start := time.Now()
		buildInfo := buildManifest[svc]
		buildHash := s.smartBuildCtrl.GetBuildHash(buildInfo, svc)

		isCached, _, err := s.imageCacheChecker.IsCached(ctx, manifestName, buildInfo.Image, buildHash, svc)
		if err != nil {
			return cachedSvcs, notCachedSvcs, err
		}
		meta.CacheHitDuration = time.Since(start)
		if isCached {
			meta.CacheHit = true
			cachedSvcs = append(cachedSvcs, svc)
		} else {
			meta.CacheHit = false
			notCachedSvcs = append(notCachedSvcs, svc)
		}
	}
	return cachedSvcs, notCachedSvcs, nil
}

func (s *SequentialCheckStrategy) CloneGlobalImagesToDev(manifestName string, buildManifest build.ManifestBuild, svcsToClone []string) error {
	skippedServices := make([]string, 0)
	for _, svc := range svcsToClone {

		meta := s.metadataCollector.GetMetadata(svc)
		ok, globalImage := s.imageCacheChecker.GetFromCache(svc)
		if !ok {
			return fmt.Errorf("image %s not found in cache", svc)
		}

		buildInfo := buildManifest[svc]
		devImage := buildInfo.Image
		if buildInfo.Dockerfile != "" && buildInfo.Image == "" {
			devImage = s.tagger.GetImageReferencesForDeploy(manifestName, svc)[0]
		}

		reference, err := s.smartBuildCtrl.CloneGlobalImageToDev(globalImage, devImage)
		if err != nil {
			return err
		}

		s.serviceEnvVarsSetter.SetServiceEnvVars(svc, reference)
		meta.Success = true
		skippedServices = append(skippedServices, svc)
	}
	if len(skippedServices) == 1 {
		s.ioCtrl.Infof("Okteto Smart Builds is skipping build of %q because it's already built from cache.", skippedServices[0])
	} else if len(skippedServices) > 1 {
		s.ioCtrl.Infof("Okteto Smart Builds is skipping build of %d services [%s] because they're already built from cache.", len(skippedServices), strings.Join(skippedServices, ", "))
	}
	return nil
}

func (s *SequentialCheckStrategy) GetImageDigestReferenceForServiceDeploy(manifestName, service string, buildInfo *build.Info) (string, error) {
	var possibleReferences []string
	if buildInfo.Dockerfile != "" && buildInfo.Image == "" {
		possibleReferences = s.tagger.GetImageReferencesForDeploy(manifestName, service)
	} else if buildInfo.Image != "" {
		possibleReferences = []string{buildInfo.Image}
	}

	for _, ref := range possibleReferences {
		imageWithDigest, err := s.imageCacheChecker.LookupReferenceWithDigest(ref)
		if err != nil {
			if oktetoErrors.IsNotFound(err) {
				continue
			}
			// return error if the registry doesn't send a not found error
			return "", fmt.Errorf("error checking image at registry %s: %w", ref, err)
		}
		return imageWithDigest, nil
	}
	return "", fmt.Errorf("images [%s] not found", strings.Join(possibleReferences, ", "))
}
