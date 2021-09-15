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
	"os"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
)

type ClusterType string

var (
	CloudCluster      ClusterType = "OktetoCloud"
	EnterpriseCluster ClusterType = "OktetoEnterprise"
	VanillaCluster    ClusterType = "Vanilla"
)

type ContextConfig struct {
	Contexts       map[string]Context `json:"Contexts"`
	CurrentContext string             `json:"CurrentContext"`
}

// Token contains the auth token and the URL it belongs to
type Context struct {
	ClusterType ClusterType `json:"ClusterType"`
	URL         string      `json:"URL"`
	Name        string      `json:"Name"`
	ApiToken    string      `json:"Token"`
}

func GetOktetoContextConfig() (*ContextConfig, error) {
	p := config.GetContextConfigPath()
	b, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields() // Force errors

	currentContext := &ContextConfig{}
	if err := dec.Decode(&currentContext); err != nil {
		return nil, err
	}

	return currentContext, nil
}

func SaveContext(clusterType ClusterType, url, name, token string) error {
	cc, err := GetOktetoContextConfig()
	if err != nil {
		log.Infof("bad token, re-initializing: %s", err)
		cc = &ContextConfig{
			Contexts: make(map[string]Context),
		}
	}

	cc.Contexts[name] = Context{
		ClusterType: clusterType,
		URL:         url,
		Name:        name,
		ApiToken:    token,
	}

	cc.CurrentContext = name

	return saveContext(cc)
}

func saveContext(c *ContextConfig) error {
	marshalled, err := json.MarshalIndent(c, "", "\t")
	if err != nil {
		log.Infof("failed to marshal context: %s", err)
		return fmt.Errorf("failed to generate your context")
	}

	contextPath := config.GetOktetoContextPath()
	if err := os.MkdirAll(contextPath, 0700); err != nil {
		log.Fatalf("failed to create %s: %s", contextPath, err)
	}

	contextConfigPath := config.GetContextConfigPath()
	if _, err := os.Stat(contextConfigPath); err == nil {
		err = os.Chmod(contextConfigPath, 0600)
		if err != nil {
			return fmt.Errorf("couldn't change token permissions: %s", err)
		}
	}

	if err := ioutil.WriteFile(contextConfigPath, marshalled, 0600); err != nil {
		return fmt.Errorf("couldn't save authentication token: %s", err)
	}

	currentToken = nil
	return nil
}

func GetCurrentContext() string {
	c, err := GetOktetoContextConfig()
	if err != nil {
		return ""
	}
	return c.CurrentContext
}

func IsOktetoCluster() bool {
	cc, err := GetOktetoContextConfig()
	if err != nil {
		return false
	}
	context := cc.Contexts[cc.CurrentContext]
	if context.ClusterType == CloudCluster || context.ClusterType == EnterpriseCluster {
		return true
	}
	return false
}
