// Copyright 2021 The Okteto Authors
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

package okteto

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/log"
)

type ContextConfig struct {
	Contexts       map[string]Context `json:"contexts"`
	CurrentContext string             `json:"current-context"`
}

// Context contains the information related to an okteto context
type Context struct {
	Name       string `json:"name,omitempty"`
	ID         string `json:"id,omitempty"`
	Username   string `json:"username,omitempty"`
	Token      string `json:"token,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	Kubeconfig string `json:"kubeconfig,omitempty"`
	Buildkit   string `json:"buildkit,omitempty"`
	Registry   string `json:"registry,omitempty"`
}

//ContextExists checks if an okteto context has been created
func ContextExists() bool {
	if _, err := os.Stat(config.GetOktetoContextFolder()); !os.IsNotExist(err) {
		return true
	}
	return false
}

// GetClusterContext returns the k8s context names given an okteto URL
func GetClusterContext() string {
	return UrlToContext(GetURL())
}

func UrlToContext(uri string) string {
	u, _ := url.Parse(uri)
	return strings.ReplaceAll(u.Host, ".", "_")
}

func IsOktetoCluster(name string) bool {
	u, err := url.Parse(name)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func GetContextConfig() (*ContextConfig, error) {
	p := config.GetOktetoContextsConfigPath()
	b, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, errors.ErrNoActiveOktetoContexts
	}

	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields() // Force errors

	ctxConfig := &ContextConfig{}
	if err := dec.Decode(&ctxConfig); err != nil {
		return nil, err
	}

	return ctxConfig, nil
}

func GetCurrentContext() string {
	c, err := GetContextConfig()
	if err != nil {
		return ""
	}
	return c.CurrentContext
}

func SetCurrentContext(name, userID, username, token, namespace, kubeconfig, buildkitURL, registryURL string) error {
	var err error
	var cc *ContextConfig
	if ContextExists() {
		cc, err = GetContextConfig()
		if err != nil {
			log.Infof("bad contexts, re-initializing: %s", err)
		}
	} else {
		cc = &ContextConfig{
			Contexts: make(map[string]Context),
		}
	}

	cc.Contexts[name] = Context{
		Name:       name,
		ID:         userID,
		Username:   username,
		Token:      token,
		Namespace:  namespace,
		Kubeconfig: kubeconfig,
		Buildkit:   buildkitURL,
		Registry:   registryURL,
	}

	cc.CurrentContext = name

	return saveContextConfigInFile(cc)
}

func saveContextConfigInFile(c *ContextConfig) error {
	marshalled, err := json.MarshalIndent(c, "", "\t")
	if err != nil {
		log.Infof("failed to marshal context: %s", err)
		return fmt.Errorf("failed to generate your context")
	}

	contextFolder := config.GetOktetoContextFolder()
	if err := os.MkdirAll(contextFolder, 0700); err != nil {
		log.Fatalf("failed to create %s: %s", contextFolder, err)
	}

	contextConfigPath := config.GetOktetoContextsConfigPath()
	if _, err := os.Stat(contextConfigPath); err == nil {
		err = os.Chmod(contextConfigPath, 0600)
		if err != nil {
			return fmt.Errorf("couldn't change context permissions: %s", err)
		}
	}

	if err := ioutil.WriteFile(contextConfigPath, marshalled, 0600); err != nil {
		return fmt.Errorf("couldn't save context: %s", err)
	}

	currentToken = nil
	return nil
}
