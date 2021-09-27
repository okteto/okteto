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

package analytics

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/denisbrodbeck/machineid"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
)

var (
	CloudContext      = "Cloud"
	StagingContext    = "Staging"
	EnterpriseContext = "Enterprise"
	KubernetesContext = "Kubernetes"
)

// Analytics contains the analytics configuration
type Analytics struct {
	Enabled   bool   `json:"enabled"`
	MachineID string `json:"machineID"`
}

func getContextType(oktetoContext string) string {
	if okteto.IsOktetoCluster(oktetoContext) {
		switch oktetoContext {
		case okteto.CloudURL:
			return CloudContext
		case okteto.StagingURL:
			return StagingContext
		default:
			return EnterpriseContext
		}
	}
	return KubernetesContext
}

func Init() error {
	if fileExists() {
		return nil
	}

	a := Analytics{
		Enabled:   true,
		MachineID: generateMachineID(),
	}

	if deprecatedFileExists() {
		defer os.RemoveAll(config.GetDeprecatedAnalyticsPath())
		a.Enabled = false
	}

	return a.save()
}

func fileExists() bool {
	if _, err := os.Stat(config.GetAnalyticsPath()); !os.IsNotExist(err) {
		return true
	}
	return false
}

func deprecatedFileExists() bool {
	if _, err := os.Stat(config.GetDeprecatedAnalyticsPath()); !os.IsNotExist(err) {
		return true
	}
	return false
}

func get() *Analytics {
	if !fileExists() {
		return &Analytics{Enabled: false, MachineID: ""}
	}
	p := config.GetAnalyticsPath()

	b, err := ioutil.ReadFile(p)
	if err != nil {
		log.Debugf("error reading analytics file: %s", err)
		return &Analytics{Enabled: false, MachineID: ""}
	}

	result := &Analytics{}
	if err := json.Unmarshal(b, result); err != nil {
		log.Debugf("error unmarshaling analytics: %s", err)
		return &Analytics{Enabled: false, MachineID: ""}
	}

	return result
}

func (a *Analytics) save() error {
	if a.MachineID == "" || a.MachineID == "na" {
		a.MachineID = generateMachineID()
	}
	marshalled, err := json.MarshalIndent(a, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to generate analytics file: %s", err)
	}

	oktetoHome := config.GetOktetoHome()
	if err := os.MkdirAll(oktetoHome, 0700); err != nil {
		log.Fatalf("failed to create %s: %s", oktetoHome, err)
	}

	analyticsPath := config.GetAnalyticsPath()
	if _, err := os.Stat(analyticsPath); err == nil {
		err = os.Chmod(analyticsPath, 0600)
		if err != nil {
			return fmt.Errorf("couldn't change analytics permissions: %s", err)
		}
	}

	if err := ioutil.WriteFile(analyticsPath, marshalled, 0600); err != nil {
		return fmt.Errorf("couldn't save analytics: %s", err)
	}

	return nil
}

// Disable disables analytics
func Disable() error {
	a := get()
	a.Enabled = false
	trackDisable(true)
	return a.save()
}

// Enable enables analytics
func Enable() error {
	a := get()
	a.Enabled = true
	return a.save()
}

func getTrackID() string {
	oktetoContextConfig, err := okteto.GetContextConfig()
	if err == nil {
		if oktetoContext, ok := oktetoContextConfig.Contexts[oktetoContextConfig.CurrentContext]; ok {
			if oktetoContext.ID != "" {
				return oktetoContext.ID
			}
		}
	}
	a := get()
	return a.MachineID
}

func generateMachineID() string {
	mid, err := machineid.ProtectedID("okteto")
	if err != nil {
		log.Infof("failed to generate a machine id: %v", err)
		mid = "na"
	}

	return mid
}
