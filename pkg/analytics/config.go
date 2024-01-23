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

package analytics

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/denisbrodbeck/machineid"
	"github.com/okteto/okteto/pkg/config"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
)

var (
	CloudContext      = "Cloud"
	StagingContext    = "Staging"
	EnterpriseContext = "Enterprise"
	KubernetesContext = "Kubernetes"
	currentAnalytics  *Analytics
)

// Analytics contains the analytics configuration
type Analytics struct {
	MachineID string `json:"machineID"`
	Enabled   bool   `json:"enabled"`
}

func getContextType(oktetoContext string) string {
	if okteto.IsOkteto() {
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
	if _, err := os.Stat(config.GetAnalyticsPath()); err != nil {
		if os.IsNotExist(err) {
			return false
		}
		oktetoLog.Fatalf("error accessing okteto config folder '%s': %s", config.GetOktetoHome(), err)
	}
	return true
}

func deprecatedFileExists() bool {
	if _, err := os.Stat(config.GetDeprecatedAnalyticsPath()); !os.IsNotExist(err) {
		return true
	}
	return false
}

func get() *Analytics {
	if currentAnalytics != nil {
		return currentAnalytics
	}

	if !fileExists() {
		return &Analytics{Enabled: false, MachineID: ""}
	}

	b, err := os.ReadFile(config.GetAnalyticsPath())
	if err != nil {
		oktetoLog.Debugf("error reading analytics file: %s", err)
		return &Analytics{Enabled: false, MachineID: ""}
	}

	result := &Analytics{}
	if err := json.Unmarshal(b, result); err != nil {
		oktetoLog.Debugf("error unmarshaling analytics: %s", err)
		return &Analytics{Enabled: false, MachineID: ""}
	}

	return result
}

func (a *Analytics) save() error {
	if currentAnalytics == nil {
		currentAnalytics = a
	}
	if a.MachineID == "" || a.MachineID == "na" {
		a.MachineID = generateMachineID()
	}
	marshalled, err := json.MarshalIndent(a, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to generate analytics file: %w", err)
	}

	oktetoHome := config.GetOktetoHome()
	if err := os.MkdirAll(oktetoHome, 0700); err != nil {
		oktetoLog.Fatalf("failed to create %s: %s", oktetoHome, err)
	}

	analyticsPath := config.GetAnalyticsPath()
	if _, err := os.Stat(analyticsPath); err == nil {
		err = os.Chmod(analyticsPath, 0600)
		if err != nil {
			return fmt.Errorf("couldn't change analytics permissions: %w", err)
		}
	}

	if err := os.WriteFile(analyticsPath, marshalled, 0600); err != nil {
		return fmt.Errorf("couldn't save analytics: %w", err)
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
	if okteto.GetContext().UserID != "" {
		return okteto.GetContext().UserID
	}
	a := get()
	return a.MachineID
}

func generateMachineID() string {
	mid, err := machineid.ProtectedID("okteto")
	if err != nil {
		oktetoLog.Infof("failed to generate a machine id: %v", err)
		mid = "na"
	}

	return mid
}
