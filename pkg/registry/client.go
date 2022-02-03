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

package registry

import (
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
)

func clientOptions(ref name.Reference) remote.Option {
	registry := ref.Context().RegistryStr()
	oktetoLog.Debugf("calling registry %s", registry)

	okRegistry := okteto.Context().Registry
	if okRegistry == registry {
		username := okteto.Context().UserID
		password := okteto.Context().Token

		authenticator := &authn.Basic{
			Username: username,
			Password: password,
		}
		return remote.WithAuth(authenticator)
	}
	return remote.WithAuthFromKeychain(authn.DefaultKeychain)
}

func digestForReference(reference string) (string, error) {
	ref, err := name.ParseReference(reference)
	if err != nil {
		return "", err
	}

	options := clientOptions(ref)

	img, err := remote.Get(ref, options)
	if err != nil {
		return "", err
	}

	return img.Digest.String(), nil
}

func configForReference(reference string) (v1.Config, error) {
	ref, err := name.ParseReference(reference)
	if err != nil {
		return v1.Config{}, err
	}

	options := clientOptions(ref)

	img, err := remote.Image(ref, options)
	if err != nil {
		oktetoLog.Debugf("error getting image from remote")
		return v1.Config{}, err
	}

	configFile, err := img.ConfigFile()
	if err != nil {
		oktetoLog.Debugf("error getting image config from remote")
		return v1.Config{}, err
	}

	return configFile.Config, nil
}

// GetReferecenceEnvs returns the values to setup the image environment variables
func GetReferecenceEnvs(reference string) (reg, repo, tag, image string) {
	ref, err := name.ParseReference(reference)
	if err != nil {
		oktetoLog.Debugf("error parsing reference: %s - %v", reference, err)
		return "", "", "", reference
	}

	return ref.Context().RegistryStr(), ref.Context().RepositoryStr(), ref.Identifier(), ref.Name()
}
