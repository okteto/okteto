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
	"context"
	"fmt"
	"strings"
	"time"

	buildTypes "github.com/okteto/okteto/cmd/build/v2/types"
	"github.com/okteto/okteto/pkg/build"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/registry"
)

// ImageTagger defines the interface for managing image tags and references.
type ImageTagger interface {
	GetGlobalTagFromDevIfNeccesary(tags, namespace, registryURL, buildHash string, ic registry.ImageCtrl) string
	GetImageReferencesForTag(manifestName, svcToBuildName, tag string) []string
	GetImageReferencesForDeploy(manifestName, svcToBuildName string) []string
}

// SmartBuildController defines the interface for smart build operations.
type SmartBuildController interface {
	GetBuildHash(buildInfo *build.Info, service string) string
	CloneGlobalImageToDev(globalImage, svcImage string) (string, error)
}

type CacheProbe interface {
	IsCached(manifestName, image, buildHash, svcToBuild string) (bool, string, error)
	LookupReferenceWithDigest(reference string) (string, error)
	GetFromCache(svc string) (hit bool, reference string)
}

type ServiceEnvVarsSetter interface {
	SetServiceEnvVars(service, reference string)
}

type SequentialCheckStrategy struct {
	tagger               ImageTagger
	hasher               hasherController
	imageCacheChecker    CacheProbe
	ioCtrl               *io.Controller
	serviceEnvVarsSetter ServiceEnvVarsSetter
	cloner               *cloner
}

func NewSequentialCheckStrategy(tagger ImageTagger, hasher hasherController, imageCacheChecker CacheProbe, ioCtrl *io.Controller, serviceEnvVarsSetter ServiceEnvVarsSetter, cloner *cloner) *SequentialCheckStrategy {
	return &SequentialCheckStrategy{
		tagger:               tagger,
		hasher:               hasher,
		imageCacheChecker:    imageCacheChecker,
		ioCtrl:               ioCtrl,
		serviceEnvVarsSetter: serviceEnvVarsSetter,
		cloner:               cloner,
	}
}

func (s *SequentialCheckStrategy) CheckServicesCache(ctx context.Context, manifestName string, buildManifest build.ManifestBuild, svcsToBuild []*buildTypes.BuildInfo) (cachedSvcs []*buildTypes.BuildInfo, notCachedSvcs []*buildTypes.BuildInfo, err error) {
	// svcsToBuild is already ordered by dependencies (from DAG.Ordered())
	// so we can optimize by stopping the check as soon as we find a service that's not cached.
	// All subsequent services that depend on it will also need to be rebuilt.

	dependantMap := make(map[string][]string)
	// Build dependantMap: for each service, track which services depend on it
	for svcName, buildInfo := range buildManifest {
		for _, dep := range buildInfo.DependsOn {
			dependantMap[dep] = append(dependantMap[dep], svcName)
		}
	}

	// Track processed services to avoid duplicates
	processed := make(map[string]bool)

	// Map of service name to BuildInfo for quick lookup
	buildInfoByName := make(map[string]*buildTypes.BuildInfo, len(svcsToBuild))
	for _, bi := range svcsToBuild {
		buildInfoByName[bi.Name()] = bi
	}

	for _, svc := range svcsToBuild {
		// Skip if already processed
		name := svc.Name()
		if processed[name] {
			continue
		}

		s.ioCtrl.SetStage(fmt.Sprintf("Building service %s", svc))
		buildInfo := buildManifest[name]

		startBuildHashTime := time.Now()
		buildHash := s.hasher.hashWithBuildContext(buildInfo, name)
		buildHashDuration := time.Since(startBuildHashTime)
		svc.SetBuildHash(buildHash, buildHashDuration)

		isCachedStartTime := time.Now()
		isCached, _, err := s.imageCacheChecker.IsCached(manifestName, buildInfo.Image, buildHash, name)
		if err != nil {
			s.ioCtrl.Logger().Infof("error checking if image is cached: %s", err)
		}
		isCachedDuration := time.Since(isCachedStartTime)
		svc.SetCacheHit(isCached, isCachedDuration)

		processed[name] = true

		if isCached {
			cachedSvcs = append(cachedSvcs, svc)

			reference, err := s.cloneGlobalImageToDev(manifestName, buildManifest, svc)
			if err != nil {
				s.ioCtrl.Logger().Infof("error cloning global image to dev: %s", err)
				return cachedSvcs, notCachedSvcs, fmt.Errorf("error cloning svc %s global image to dev: %w", svc.Name(), err)
			}
			s.serviceEnvVarsSetter.SetServiceEnvVars(svc.Name(), reference)
		} else {
			notCachedSvcs = append(notCachedSvcs, svc)

			// Recursively add all dependent services to notCached without checking cache
			notCachedSvcs = s.addDependentsToNotCached(svc, dependantMap, processed, notCachedSvcs, buildInfoByName)
		}
	}
	if len(cachedSvcs) == 1 {
		s.ioCtrl.Out().Infof("Okteto Smart Builds is skipping build of %q because it's already built from cache.", cachedSvcs[0])
	} else if len(cachedSvcs) > 1 {
		s.ioCtrl.Out().Infof("Okteto Smart Builds is skipping build of %d services [%v] because they're already built from cache.", len(cachedSvcs), cachedSvcs)
	}
	return cachedSvcs, notCachedSvcs, nil
}

func (s *SequentialCheckStrategy) cloneGlobalImageToDev(manifestName string, buildManifest build.ManifestBuild, svc *buildTypes.BuildInfo) (string, error) {
	buildInfo := buildManifest[svc.Name()]
	devImage := buildInfo.Image
	if buildInfo.Dockerfile != "" && buildInfo.Image == "" {
		devImage = s.tagger.GetImageReferencesForDeploy(manifestName, svc.Name())[0]
	}
	ok, globalImage := s.imageCacheChecker.GetFromCache(svc.Name())
	if !ok {
		return "", fmt.Errorf("image %s not found in cache", svc.Name())
	}
	cloneStartTime := time.Now()
	reference, err := s.cloner.CloneGlobalImageToDev(globalImage, devImage)
	cloneDuration := time.Since(cloneStartTime)
	svc.SetCloneDuration(cloneDuration, err == nil)

	return reference, nil
}

// addDependentsToNotCached recursively adds all dependent services to notCached
// This ensures that when a service is not cached, all services that depend on it
// (directly or indirectly) are also marked as not cached without checking cache
func (s *SequentialCheckStrategy) addDependentsToNotCached(svc *buildTypes.BuildInfo, dependantMap map[string][]string, processed map[string]bool, notCachedSvcs []*buildTypes.BuildInfo, buildInfoByName map[string]*buildTypes.BuildInfo) []*buildTypes.BuildInfo {
	for _, dependent := range dependantMap[svc.Name()] {
		if !processed[dependent] {
			processed[dependent] = true
			bi, ok := buildInfoByName[dependent]
			if ok {
				notCachedSvcs = append(notCachedSvcs, bi)
			}
			// Recursively add dependents of this dependent
			notCachedSvcs = s.addDependentsToNotCached(bi, dependantMap, processed, notCachedSvcs, buildInfoByName)
		}
	}

	return notCachedSvcs
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
