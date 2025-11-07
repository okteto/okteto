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
	"sync"
	"time"

	buildTypes "github.com/okteto/okteto/cmd/build/v2/types"
	"github.com/okteto/okteto/pkg/build"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log/io"
)

type ParallelCheckStrategy struct {
	tagger               ImageTagger
	hasher               hasherController
	imageCacheChecker    CacheProbe
	ioCtrl               *io.Controller
	serviceEnvVarsSetter ServiceEnvVarsSetter
	cloner               *cloner
}

func NewParallelCheckStrategy(tagger ImageTagger, hasher hasherController, imageCacheChecker CacheProbe, ioCtrl *io.Controller, serviceEnvVarsSetter ServiceEnvVarsSetter, cloner *cloner) *ParallelCheckStrategy {
	return &ParallelCheckStrategy{
		tagger:               tagger,
		hasher:               hasher,
		imageCacheChecker:    imageCacheChecker,
		ioCtrl:               ioCtrl,
		serviceEnvVarsSetter: serviceEnvVarsSetter,
		cloner:               cloner,
	}
}

type node struct {
	s        *buildTypes.BuildInfo
	done     chan struct{}
	isCached bool
	err      error
}

func (s *ParallelCheckStrategy) CheckServicesCache(ctx context.Context, manifestName string, buildManifest build.ManifestBuild, svcsToBuild []*buildTypes.BuildInfo) (cachedSvcs []*buildTypes.BuildInfo, notCachedSvcs []*buildTypes.BuildInfo, err error) {
	// svcsToBuild is already ordered by dependencies (from DAG.Ordered())
	// cachedSvcs and notCachedSvcs needs to follow the same order as svcsToBuild

	// Build dependantMap: for each service, track which services depend on it
	dependantMap := make(map[string][]string)
	for svcName, buildInfo := range buildManifest {
		for _, dep := range buildInfo.DependsOn {
			dependantMap[svcName] = append(dependantMap[svcName], dep)
		}
	}

	nodes := make(map[string]*node, len(svcsToBuild))
	for _, svc := range svcsToBuild {
		nodes[svc.Name()] = &node{s: svc, done: make(chan struct{})}
	}

	sp := s.ioCtrl.Out().Spinner("Checking if the images are already built from cache...")

	var wg sync.WaitGroup
	wg.Add(len(svcsToBuild))
	for _, svc := range svcsToBuild {
		svc := svc
		go func(svc *buildTypes.BuildInfo) {
			defer wg.Done()

			depNotCached := false
			// Wait until all dependencies are ready
			for _, dep := range dependantMap[svc.Name()] {
				n := nodes[dep]
				<-n.done
				if !n.isCached {
					depNotCached = true
					break
				}
			}

			n := nodes[svc.Name()]
			if depNotCached {
				n.isCached = false
				close(n.done)
				return
			}

			buildInfo := buildManifest[svc.Name()]
			startBuildHashTime := time.Now()
			buildHash := s.hasher.hashWithBuildContext(buildInfo, svc.Name())
			buildHashDuration := time.Since(startBuildHashTime)
			svc.SetBuildHash(buildHash, buildHashDuration)

			isCachedStartTime := time.Now()
			isCached, cachedImage, err := s.imageCacheChecker.IsCached(manifestName, buildInfo.Image, buildHash, svc.Name())
			if err != nil {
				// Log the error and continue adding it to the notCachedSvcs list
				s.ioCtrl.Logger().Infof("error checking if image is cached: %s", err)
			}
			isCachedDuration := time.Since(isCachedStartTime)
			svc.SetCacheHit(isCached, isCachedDuration)

			if isCached {
				reference, err := s.cloneGlobalImageToDev(cachedImage, manifestName, buildManifest, svc)
				if err != nil {
					s.ioCtrl.Logger().Infof("error cloning global image to dev: %s", err)
					n.err = fmt.Errorf("error cloning svc %s global image to dev: %w", svc.Name(), err)
					close(n.done)
					return
				}

				sp.Stop()
				s.ioCtrl.Out().Infof("Okteto Smart Builds is skipping build of %q because it's already built from cache.", svc.Name())
				sp.Start()
				s.serviceEnvVarsSetter.SetServiceEnvVars(svc.Name(), reference)
			}

			n.isCached = isCached
			close(n.done)
		}(svc)
	}
	wg.Wait()

	for _, svc := range svcsToBuild {
		n := nodes[svc.Name()]
		if n.err != nil {
			return cachedSvcs, notCachedSvcs, n.err
		}
		if n.isCached {
			cachedSvcs = append(cachedSvcs, n.s)
		} else {
			notCachedSvcs = append(notCachedSvcs, n.s)
		}
	}
	return cachedSvcs, notCachedSvcs, nil
}

func (s *ParallelCheckStrategy) cloneGlobalImageToDev(cachedImage, manifestName string, buildManifest build.ManifestBuild, svc *buildTypes.BuildInfo) (string, error) {
	buildInfo := buildManifest[svc.Name()]
	devImage := buildInfo.Image
	if buildInfo.Dockerfile != "" && buildInfo.Image == "" {
		devImage = s.tagger.GetImageReferencesForDeploy(manifestName, svc.Name())[0]
	}
	cloneStartTime := time.Now()
	reference, err := s.cloner.CloneGlobalImageToDev(cachedImage, devImage)
	cloneDuration := time.Since(cloneStartTime)
	svc.SetCloneDuration(cloneDuration, err == nil)

	return reference, err
}

func (s *ParallelCheckStrategy) GetImageDigestReferenceForServiceDeploy(manifestName, service string, buildInfo *build.Info) (string, error) {
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
