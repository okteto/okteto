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

package discovery

import (
	"errors"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
)

var (
	// ErrComposeFileNotFound is raised when discovery package could not found any compose file
	ErrComposeFileNotFound = oktetoErrors.UserError{
		E:    errors.New("could not detect any compose file"),
		Hint: "If you have a compose file, use the flag '--file' to point to your compose file",
	}
	// ErrOktetoManifestNotFound is raised when discovery package could not found any okteto manifest file
	ErrOktetoManifestNotFound = oktetoErrors.UserError{
		E:    errors.New("could not detect any okteto manifest"),
		Hint: "If you have an okteto manifest file, use the flag '--file' to point to your okteto manifest file",
	}
	// ErrOktetoPipelineManifestNotFound is raised when discovery package could not found any okteto pipeline manifest
	ErrOktetoPipelineManifestNotFound = errors.New("could not detect any okteto pipeline manifest")
	// ErrHelmChartNotFound is raised when discovery package could not found any helm chart
	ErrHelmChartNotFound = errors.New("could not detect any helm chart")
	// ErrK8sManifestNotFound is raised when discovery package could not found any k8s manifest
	ErrK8sManifestNotFound = errors.New("could not detect any k8s manifest")
)
