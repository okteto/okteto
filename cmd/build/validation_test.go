// Copyright 2022 The Okteto Authors
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
	"context"
	"testing"

	"github.com/okteto/okteto/internal/test"
	"github.com/stretchr/testify/assert"
)

func TestAllServicesAlreadyBuilt(t *testing.T) {
	fakeReg := test.NewFakeOktetoRegistry(nil)
	bc := &Command{
		GetManifest: getFakeManifest,
		Registry:    fakeReg,
	}
	alreadyBuilt := []string{}
	fakeReg.AddImageByName(alreadyBuilt...)
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest)
	//should not throw error
	assert.NoError(t, err)
	assert.Equal(t, len(fakeManifest.Build)-len(alreadyBuilt), len(toBuild))
}

func TestServicesNotAreAlreadyBuilt(t *testing.T) {
	fakeReg := test.NewFakeOktetoRegistry(nil)
	bc := &Command{
		GetManifest: getFakeManifest,
		Registry:    fakeReg,
	}
	alreadyBuilt := []string{"test/test-1"}
	fakeReg.AddImageByName(alreadyBuilt...)
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest)
	//should not throw error
	assert.NoError(t, err)
	assert.Equal(t, len(fakeManifest.Build)-len(alreadyBuilt), len(toBuild))
}

func TestNoServiceBuilt(t *testing.T) {
	fakeReg := test.NewFakeOktetoRegistry(nil)
	bc := &Command{
		GetManifest: getFakeManifest,
		Registry:    fakeReg,
	}
	alreadyBuilt := []string{"test/test-1", "test/test-2"}
	fakeReg.AddImageByName(alreadyBuilt...)
	ctx := context.Background()
	toBuild, err := bc.GetServicesToBuild(ctx, fakeManifest)
	//should not throw error
	assert.NoError(t, err)
	assert.Equal(t, len(fakeManifest.Build)-len(alreadyBuilt), len(toBuild))
}
