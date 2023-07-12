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

	"github.com/docker/docker-credential-helpers/credentials"
)

var ErrNotImplemented = errors.New("not implemented")

type RegistryCredentialsGetter interface {
	GetRegistryCredentials(host string) (string, string, error)
}

type OktetoClusterHelper struct {
	getter RegistryCredentialsGetter
}

var _ credentials.Helper = (*OktetoClusterHelper)(nil)

func NewOktetoClusterHelper(getter RegistryCredentialsGetter) *OktetoClusterHelper {
	return &OktetoClusterHelper{getter: getter}
}

// Add appends credentials to the store.
func (*OktetoClusterHelper) Add(*credentials.Credentials) error {
	return ErrNotImplemented
}

// Delete removes credentials from the store.
func (*OktetoClusterHelper) Delete(_ string) error {
	return ErrNotImplemented
}

// Get retrieves credentials from the store.
// It returns username and secret as strings.
func (och *OktetoClusterHelper) Get(serverURL string) (string, string, error) {
	return och.getter.GetRegistryCredentials(serverURL)
}

// List returns the stored serverURLs and their associated usernames.
func (*OktetoClusterHelper) List() (map[string]string, error) {
	return nil, ErrNotImplemented
}
