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
	"path"

	"github.com/docker/cli/cli/config"
	dockertypes "github.com/docker/cli/cli/config/types"
	"github.com/docker/docker-credential-helpers/credentials"
	okconfig "github.com/okteto/okteto/pkg/config"
)

var ErrNotImplemented = errors.New("not implemented")

type RegistryCredentialsGetter interface {
	GetRegistryCredentials(host string) (string, string, error)
}

type OktetoClusterHelper struct {
	getter  RegistryCredentialsGetter
	dirname string
}

var _ credentials.Helper = (*OktetoClusterHelper)(nil)

const oktetoConfigFilename = "regcreds-tmp"

func NewOktetoClusterHelper(getter RegistryCredentialsGetter) *OktetoClusterHelper {
	h := okconfig.GetOktetoHome()
	return &OktetoClusterHelper{
		getter:  getter,
		dirname: path.Join(h, oktetoConfigFilename),
	}
}

// Add appends credentials to the store.
func (o *OktetoClusterHelper) Add(reg *credentials.Credentials) error {
	cf, err := config.Load(o.dirname)
	if err != nil {
		return err
	}
	cf.AuthConfigs[reg.ServerURL] = dockertypes.AuthConfig{
		Username: reg.Username,
		Password: reg.Secret,
	}
	return cf.Save()
}

// Delete removes credentials from the store.
func (o *OktetoClusterHelper) Delete(regHost string) error {
	cf, err := config.Load(o.dirname)
	if err != nil {
		return err
	}
	delete(cf.AuthConfigs, regHost)
	return cf.Save()
}

// Get retrieves credentials from the store.
// It returns username and secret as strings.
func (o *OktetoClusterHelper) Get(serverURL string) (string, string, error) {
	cf, err := config.Load(o.dirname)
	if err != nil {
		return "", "", err
	}
	if creds, ok := cf.AuthConfigs[serverURL]; ok {
		return creds.Username, creds.Password, nil
	}
	return o.getter.GetRegistryCredentials(serverURL)
}

// List returns the stored serverURLs and their associated usernames.
func (*OktetoClusterHelper) List() (map[string]string, error) {
	return nil, ErrNotImplemented
}
