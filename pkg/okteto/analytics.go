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

func ContextType(oktetoContext string) string {
	if IsOktetoContext(oktetoContext) {
		switch oktetoContext {
		case CloudURL:
			return CloudContext
		case StagingURL:
			return StagingContext
		default:
			return EnterpriseContext
		}
	}
	return KubernetesContext

}

//AnalyticsExists checks if the analytics file has been created
func AnalyticsExists() bool {
	if _, err := os.Stat(config.GetAnalyticsPath()); !os.IsNotExist(err) {
		return true
	}
	return false
}

//AnalyticsDeprecatedExists checks if the .noanalytics file has been created
func AnalyticsDeprecatedExists() bool {
	if _, err := os.Stat(config.GetDeprecatedAnalyticsPath()); !os.IsNotExist(err) {
		return true
	}
	return false
}

func (a *Analytics) Save() error {
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

//GetAnalytics returns the analytics configuration
func GetAnalytics() *Analytics {
	if !AnalyticsExists() {
		return &Analytics{Enabled: true, MachineID: ""}
	}
	p := config.GetAnalyticsPath()

	b, err := ioutil.ReadFile(p)
	if err != nil {
		log.Debug("error reading analytics file: %s", err)
		return &Analytics{Enabled: false, MachineID: ""}
	}

	result := &Analytics{}
	if err := json.Unmarshal(b, result); err != nil {
		log.Debug("error unmarshaling analytics: %s", err)
		return &Analytics{Enabled: false, MachineID: ""}
	}

	return result
}

// SaveMachineID updates the token file with the machineID value
func SaveMachineID(machineID string) error {
	t, err := GetToken()
	if err != nil {
		log.Infof("bad token, re-initializing: %s", err)
		t = &Token{}
	}

	t.MachineID = machineID
	return save(t)
}
