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
	"sync"

	"github.com/denisbrodbeck/machineid"
	"github.com/okteto/okteto/pkg/config"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
)

var (
	EnterpriseContext = "Enterprise"
	KubernetesContext = "Kubernetes"
	currentAnalytics  *Analytics
	// analyticsMu guards currentAnalytics so the memoized load in get() and the
	// write in save() never race.
	analyticsMu sync.Mutex
)

// Analytics contains the analytics configuration
type Analytics struct {
	MachineID string `json:"machineID"`
	Enabled   bool   `json:"enabled"`
}

func getContextType() string {
	if okteto.IsOkteto() {
		return EnterpriseContext
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

// get returns the analytics config, memoizing the first successful load so later
// calls skip the disk read. Access to the shared *Analytics is guarded by
// analyticsMu, so concurrent callers are safe. Disabled fallbacks (missing file
// / read error) are returned fresh, not cached.
func get() *Analytics {
	analyticsMu.Lock()
	defer analyticsMu.Unlock()

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

	currentAnalytics = result
	return currentAnalytics
}

func (a *Analytics) save() error {
	analyticsMu.Lock()
	if currentAnalytics == nil {
		currentAnalytics = a
	}
	analyticsMu.Unlock()
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

// Disable disables analytics. trackDisable must run before flipping Enabled:
// get() returns the shared *Analytics that the tracking path also reads, so
// disabling first would make analyticsEnabled() short-circuit and drop the event.
func Disable() error {
	a := get()
	trackDisable(true)
	a.Enabled = false
	return a.save()
}

// Enable enables analytics. Like Disable, it mutates the shared *Analytics from
// get() (see Disable for the concurrency invariant).
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

// analyticsEnabled returns true if analytics should be sent.
// Both mixpanelBackend and posthogBackend call this to stay in sync.
// Order matters: IsContextInitialized must precede disabledByOktetoAdmin
// because the latter calls GetContext() which panics on uninitialized context.
func analyticsEnabled() bool {
	return get().Enabled && okteto.IsContextInitialized() && !disabledByOktetoAdmin()
}

func generateMachineID() string {
	mid, err := machineid.ProtectedID("okteto")
	if err != nil {
		oktetoLog.Infof("failed to generate a machine id: %v", err)
		mid = "na"
	}

	return mid
}
