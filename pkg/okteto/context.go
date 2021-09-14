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

// Token contains the auth token and the URL it belongs to
type Context struct {
	ClusterType ClusterType `json:"ClusterType"`
	URL         string      `json:"URL"`
	Name        string      `json:"Name"`
}

func GetOktetoContextConfig() (*Context, error) {
	p := config.GetContextConfigPath()
	b, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, err
	}

	currentContext := &Context{}
	if err := json.Unmarshal(b, currentContext); err != nil {
		return nil, err
	}

	return currentContext, nil
}

func SaveContext(clusterType ClusterType, url, name string) error {
	c, err := GetOktetoContextConfig()
	if err != nil {
		log.Infof("bad token, re-initializing: %s", err)
		c = &Context{}
	}

	c.ClusterType = clusterType
	c.URL = url
	c.Name = name

	return saveContext(c)
}

func saveContext(c *Context) error {
	marshalled, err := json.Marshal(c)
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
	return c.Name
}

func IsOktetoCluster() bool {
	c, err := GetOktetoContextConfig()
	if err != nil {
		return false
	}
	if c.ClusterType == CloudCluster || c.ClusterType == EnterpriseCluster {
		return true
	}
	return false
}
