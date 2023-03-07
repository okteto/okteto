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

package fake

import containerv1 "github.com/google/go-containerregistry/pkg/v1"

// FakeClient has everything needed to set up a test faking API calls
type FakeClient struct {
	GetImageDigest GetDigest
	GetConfig      GetConfig
}

// GetDigest has everything needed to mock a getDigest API call
type GetDigest struct {
	Result string
	Err    error
}

// GetConfig has everything needed to mock a getConfig API call
type GetConfig struct {
	Result *containerv1.ConfigFile
	Err    error
}

func (fc FakeClient) GetDigest(image string) (string, error) {
	return fc.GetImageDigest.Result, fc.GetImageDigest.Err
}

func (fc FakeClient) GetImageConfig(image string) (*containerv1.ConfigFile, error) {
	return fc.GetConfig.Result, fc.GetConfig.Err
}
