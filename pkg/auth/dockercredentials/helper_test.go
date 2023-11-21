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

package dockercredentials

import (
	"errors"
	"os"
	"testing"

	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/stretchr/testify/require"
)

var errNotFound = errors.New("fake creds not found")

type fakeGetter struct {
	creds map[string][2]string
}

func (fg fakeGetter) GetRegistryCredentials(host string) (string, string, error) {
	if fg.creds == nil {
		fg.creds = make(map[string][2]string)
	}

	if creds, ok := fg.creds[host]; ok {
		return creds[0], creds[1], nil
	}

	return "", "", errNotFound

}

func TestAddDelete(t *testing.T) {
	dir, err := os.MkdirTemp("", "")
	require.NoError(t, err)

	defer os.RemoveAll(dir)

	h := OktetoClusterHelper{
		dirname: dir,
		getter:  fakeGetter{},
	}
	creds := &credentials.Credentials{
		ServerURL: "registry.com",
		Username:  "lebowski",
		Secret:    "thedude",
	}

	err = h.Add(creds)
	require.NoError(t, err)
	user, pass, err := h.Get("registry.com")
	require.NoError(t, err)
	require.Equal(t, "lebowski", user)
	require.Equal(t, "thedude", pass)

	err = h.Delete("registry.com")
	require.NoError(t, err)

	_, _, err = h.Get("registry.com")
	require.ErrorIs(t, err, errNotFound)
}
