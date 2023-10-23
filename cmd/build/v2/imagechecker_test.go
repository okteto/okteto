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

package v2

import (
	"testing"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_checkIfBuildHashIsBuilt(t *testing.T) {

	tests := []struct {
		name          string
		imageChecker  *imageChecker
		manifestName  string
		serviceName   string
		buildHash     string
		expectedTag   string
		expectedBuilt bool
	}{
		{
			name:          "empty build hash",
			imageChecker:  &imageChecker{},
			expectedTag:   "",
			expectedBuilt: false,
		},
		{
			name: "not found hash",
			imageChecker: &imageChecker{
				tagger: imageTagger{
					cfg: &fakeConfig{},
				},
				lookupReferenceWithDigest: func(_ string, _ registryImageCheckerInterface) (string, error) {
					return "", oktetoErrors.ErrNotFound
				},
			},
			manifestName:  "manifest",
			serviceName:   "service",
			buildHash:     "buildHash",
			expectedTag:   "",
			expectedBuilt: false,
		},
		{
			name: "error getting SHA from registry",
			imageChecker: &imageChecker{
				tagger: imageTagger{
					cfg: &fakeConfig{},
				},
				lookupReferenceWithDigest: func(_ string, _ registryImageCheckerInterface) (string, error) {
					return "", assert.AnError
				},
			},
			manifestName:  "manifest",
			serviceName:   "service",
			buildHash:     "buildHash",
			expectedTag:   "",
			expectedBuilt: false,
		},
		{
			name: "found SHA from registry",
			imageChecker: &imageChecker{
				tagger: imageTagger{
					cfg: &fakeConfig{},
				},
				lookupReferenceWithDigest: func(_ string, _ registryImageCheckerInterface) (string, error) {
					return "image-tag-from-registry", nil
				},
			},
			manifestName:  "manifest",
			serviceName:   "service",
			buildHash:     "buildHash",
			expectedTag:   "image-tag-from-registry",
			expectedBuilt: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tag, isbuilt := tt.imageChecker.checkIfBuildHashIsBuilt(tt.manifestName, tt.serviceName, tt.buildHash)

			require.Equal(t, tt.expectedTag, tag)
			require.Equal(t, tt.expectedBuilt, isbuilt)
		})
	}
}
