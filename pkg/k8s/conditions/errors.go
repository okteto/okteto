// Copyright 2026 The Okteto Authors
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

package conditions

import (
	"fmt"
	"regexp"
	"strings"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
)

// FailedCreateError returns a user-facing error for a failed pod creation message.
func FailedCreateError(errorMessage string, dev *model.Dev) error {
	if strings.Contains(errorMessage, "exceeded quota") {
		oktetoLog.Infof("%s: %s", oktetoErrors.ErrQuota, errorMessage)
		if strings.Contains(errorMessage, "requested: pods=") {
			return fmt.Errorf("quota exceeded, you have reached the maximum number of pods per namespace")
		}
		if strings.Contains(errorMessage, "requested: requests.storage=") {
			return fmt.Errorf("quota exceeded, you have reached the maximum storage per namespace")
		}
		return oktetoErrors.ErrQuota
	}

	if IsResourcesRelatedError(errorMessage) {
		return ResourceLimitError(errorMessage, dev)
	}

	return fmt.Errorf("%s", errorMessage)
}

// IsResourcesRelatedError returns true when the message refers to resource limit validation.
func IsResourcesRelatedError(errorMessage string) bool {
	return strings.Contains(errorMessage, "maximum cpu usage") || strings.Contains(errorMessage, "maximum memory usage")
}

// ResourceLimitError formats a resource limit validation error for the user.
func ResourceLimitError(errorMessage string, dev *model.Dev) error {
	var errorToReturn string
	const regexMaxSubstring = 2

	if strings.Contains(errorMessage, "maximum cpu usage") {
		cpuMaximumRegex := regexp.MustCompile(`cpu usage per Pod is (\d*\w*)`)
		maximumCPUPerPodMatchGroups := cpuMaximumRegex.FindStringSubmatch(errorMessage)
		if len(maximumCPUPerPodMatchGroups) < regexMaxSubstring {
			errorToReturn += "The value of resources.limits.cpu in your okteto manifest exceeds the maximum CPU limit per pod. "
		} else {
			var manifestCPU string
			if limitCPU, ok := dev.Resources.Limits[apiv1.ResourceCPU]; ok {
				manifestCPU = limitCPU.String()
			}
			maximumCPUPerPod := maximumCPUPerPodMatchGroups[1]
			errorToReturn += fmt.Sprintf("The value of resources.limits.cpu in your okteto manifest (%s) exceeds the maximum CPU limit per pod (%s). ", manifestCPU, maximumCPUPerPod)
		}
	}

	if strings.Contains(errorMessage, "maximum memory usage") {
		memoryMaximumRegex := regexp.MustCompile(`memory usage per Pod is (\d*\w*)`)
		maximumMemoryPerPodMatchGroups := memoryMaximumRegex.FindStringSubmatch(errorMessage)
		if len(maximumMemoryPerPodMatchGroups) < regexMaxSubstring {
			errorToReturn += "The value of resources.limits.memory in your okteto manifest exceeds the maximum memory limit per pod."
		} else {
			var manifestMemory string
			if limitMemory, ok := dev.Resources.Limits[apiv1.ResourceMemory]; ok {
				manifestMemory = limitMemory.String()
			}
			maximumMemoryPerPod := maximumMemoryPerPodMatchGroups[1]
			errorToReturn += fmt.Sprintf("The value of resources.limits.memory in your okteto manifest (%s) exceeds the maximum memory limit per pod (%s). ", manifestMemory, maximumMemoryPerPod)
		}
	}

	return fmt.Errorf("%s", strings.TrimSpace(errorToReturn))
}
