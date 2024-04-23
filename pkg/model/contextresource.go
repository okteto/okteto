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

package model

import (
	"fmt"
	"os"

	"github.com/okteto/okteto/pkg/discovery"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/spf13/afero"
	yaml "gopkg.in/yaml.v3"
)

// ContextResource provides the context and namespace to operate within a manifest
type ContextResource struct {
	Context   string `yaml:"context"`
	Namespace string `yaml:"namespace"`
}

// TODO: remove this function once we remove the support for 'context' and 'namespace' in the okteto manifest and docker compose
func displayDeprecationWarningForContextAndNamespace(context, namespace string) {
	var s1, s2 string

	if context == "" && namespace == "" {
		return
	} else if context != "" && namespace != "" {
		s1 = "'context' and 'namespace'"
		s2 = fmt.Sprintf("'%s' and '%s'", OktetoContextEnvVar, OktetoNamespaceEnvVar)
	} else if context != "" {
		s1 = "'context'"
		s2 = fmt.Sprintf("'%s'", OktetoContextEnvVar)
	} else if namespace != "" {
		s1 = "'namespace'"
		s2 = fmt.Sprintf("'%s'", OktetoNamespaceEnvVar)
	}

	msg := "Setting %s in the okteto manifest or docker compose is deprecated. We recommend using the env var %s instead."
	oktetoLog.Warning(fmt.Sprintf(msg, s1, s2))
}

// GetContextResource returns a ContextResource object from a given file
func GetContextResource(path string) (*ContextResource, error) {
	if !filesystem.FileExistsAndNotDir(path, afero.NewOsFs()) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		path, err = discovery.GetContextResourcePath(cwd)
		if err != nil {
			return nil, err
		}
	}
	ctxResource := &ContextResource{}
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(bytes, ctxResource); err != nil {
		return nil, newManifestFriendlyError(err)
	}

	displayDeprecationWarningForContextAndNamespace(ctxResource.Context, ctxResource.Namespace)

	ctxResource.Context = os.ExpandEnv(ctxResource.Context)
	ctxResource.Namespace = os.ExpandEnv(ctxResource.Namespace)

	return ctxResource, nil
}

// UpdateContext updates the context from the resource
func (c *ContextResource) UpdateContext(okCtx string) error {
	if c.Context != "" {
		if okCtx != "" && okCtx != c.Context {
			return oktetoErrors.ErrContextNotMatching
		}
		return nil
	}

	c.Context = okCtx
	return nil
}

// UpdateNamespace updates the namespace from the resource
func (c *ContextResource) UpdateNamespace(okNs string) error {
	if c.Namespace != "" {
		if okNs != "" && c.Namespace != okNs {
			return oktetoErrors.ErrNamespaceNotMatching
		}
		return nil
	}

	c.Namespace = okNs
	return nil
}
