// Copyright 2024 The Okteto Authors
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

package env

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakeEnvManager struct {
	t           *testing.T
	maskedWords []string
	logs        []string
}

func (e *fakeEnvManager) LookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}
func (e *fakeEnvManager) SetEnv(key, value string) error {
	e.t.Setenv(key, value)
	return nil
}
func (e *fakeEnvManager) MaskVar(value string) {
	e.maskedWords = append(e.maskedWords, value)
}
func (e *fakeEnvManager) WarningLog(msg string, _ ...interface{}) {
	e.logs = append(e.logs, msg)
}

func newFakeEnvManager(t *testing.T) *fakeEnvManager {
	return &fakeEnvManager{
		t: t,
	}
}

func varExists(key string) bool {
	_, exists := os.LookupEnv(key)
	return exists
}

func Test_EnvManager(t *testing.T) {
	fakeEnvManager := newFakeEnvManager(t)

	t.Run("empty env manager", func(t *testing.T) {
		envManager := NewEnvManager(fakeEnvManager)
		err := envManager.Export()
		assert.NoError(t, err)

		var emptyGroup []Var

		envManager.AddGroup(emptyGroup, PriorityVarFromLocal)
		err = envManager.Export()
		assert.NoError(t, err)
	})

	t.Run("add multiple groups and lookup var with higher priority successfully", func(t *testing.T) {
		fakeGroupVarsFromPlatform := []Var{
			{
				Name:  "TEST_VAR_1",
				Value: "platform-value1",
			},
			{
				Name:  "TEST_VAR_2",
				Value: "platform-value2",
			},
		}

		fakeGroupVarsFromManifest := []Var{
			{
				Name:  "TEST_VAR_1",
				Value: "manifest-value1",
			},
		}

		fakeGroupVarsFromLoal := []Var{
			{
				Name:  "TEST_VAR_1",
				Value: "local-value1",
			},
		}

		fakeGroupVarsFromFlag := []Var{
			{
				Name:  "TEST_VAR_1",
				Value: "flag-value1",
			},
		}

		// making sure these vars are not set
		assert.Equal(t, false, varExists("TEST_VAR_1"))
		assert.Equal(t, false, varExists("TEST_VAR_2"))
		assert.Equal(t, false, varExists("TEST_VAR_3"))

		envManager := NewEnvManager(fakeEnvManager)
		envManager.AddGroup(fakeGroupVarsFromPlatform, PriorityVarFromPlatform)
		assert.NoError(t, envManager.Export())
		assert.Equal(t, "platform-value1", os.Getenv("TEST_VAR_1"))

		envManager.AddGroup(fakeGroupVarsFromManifest, PriorityVarFromManifest)

		// until we export, the value stays the same
		assert.Equal(t, "platform-value1", os.Getenv("TEST_VAR_1"))

		assert.NoError(t, envManager.Export())
		assert.Equal(t, "manifest-value1", os.Getenv("TEST_VAR_1"))

		envManager.AddGroup(fakeGroupVarsFromLoal, PriorityVarFromLocal)
		assert.NoError(t, envManager.Export())
		assert.Equal(t, "local-value1", os.Getenv("TEST_VAR_1"))

		envManager.AddGroup(fakeGroupVarsFromFlag, PriorityVarFromFlag)
		assert.NoError(t, envManager.Export())
		assert.Equal(t, "flag-value1", os.Getenv("TEST_VAR_1"))

		// no other groups override the value
		assert.Equal(t, "platform-value2", os.Getenv("TEST_VAR_2"))

		// make sure values are obfuscated
		// note: currently local vars are not obsucated so "local-value1" should not be in the list
		expectedMaskedValues := []string{"platform-value1", "platform-value2", "manifest-value1", "flag-value1"}
		assert.ElementsMatch(t, expectedMaskedValues, fakeEnvManager.maskedWords, "Masked words should match expected values")
	})
}

func Test_CreateGroupFromLocalVars(t *testing.T) {
	t.Run("create group from local vars", func(t *testing.T) {
		fakeLocalVars := []string{
			"TEST_VAR_1=local-value1",
			"TEST_VAR_2=local-value2",
		}

		fakeLocalGroup := CreateGroupFromLocalVars(func() []string {
			return fakeLocalVars
		})

		assert.Equal(t, 2, len(fakeLocalGroup))
		assert.ElementsMatch(t, []Var{
			{
				Name:  "TEST_VAR_1",
				Value: "local-value1",
			},
			{
				Name:  "TEST_VAR_2",
				Value: "local-value2",
			},
		}, fakeLocalGroup)
	})
}
