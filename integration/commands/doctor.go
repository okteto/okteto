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

package commands

import (
	"archive/zip"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model"
)

// DoctorOptions defines the options that can be added to a doctor command
type DoctorOptions struct {
	Workdir      string
	ManifestPath string
	Namespace    string
	OktetoHome   string
	Token        string
	DevName      string
}

// RunOktetoDoctor runs an okteto doctor command and returns the zip file path
func RunOktetoDoctor(oktetoPath string, doctorOptions *DoctorOptions) (string, error) {
	cmd := getDoctorCmd(oktetoPath, doctorOptions)
	log.Printf("Running doctor command: %s", cmd.String())

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("okteto doctor failed: %v - %s", err, string(output))
		return "", fmt.Errorf("okteto doctor failed: %w - %s", err, string(output))
	}

	// Parse the output to get the zip file path
	// The output contains: "Your doctor file is available at <path>"
	outputStr := string(output)
	zipPath := extractZipPathFromOutput(outputStr, doctorOptions.Workdir)
	if zipPath == "" {
		return "", fmt.Errorf("could not find zip file path in doctor output: %s", outputStr)
	}

	log.Printf("okteto doctor success: %s", zipPath)
	return zipPath, nil
}

func getDoctorCmd(oktetoPath string, doctorOptions *DoctorOptions) *exec.Cmd {
	cmd := exec.Command(oktetoPath, "doctor")
	cmd.Env = os.Environ()

	if doctorOptions.Workdir != "" {
		cmd.Dir = doctorOptions.Workdir
	}

	if doctorOptions.DevName != "" {
		cmd.Args = append(cmd.Args, doctorOptions.DevName)
	}

	if doctorOptions.ManifestPath != "" {
		cmd.Args = append(cmd.Args, "-f", doctorOptions.ManifestPath)
	}

	if doctorOptions.Namespace != "" {
		cmd.Args = append(cmd.Args, "--namespace", doctorOptions.Namespace)
	}

	if v := os.Getenv(model.OktetoURLEnvVar); v != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoURLEnvVar, v))
	}

	if doctorOptions.OktetoHome != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", constants.OktetoHomeEnvVar, doctorOptions.OktetoHome))
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", constants.KubeConfigEnvVar, filepath.Join(doctorOptions.OktetoHome, ".kube", "config")))
	}

	if doctorOptions.Token != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", model.OktetoTokenEnvVar, doctorOptions.Token))
	}

	return cmd
}

func extractZipPathFromOutput(output, workdir string) string {
	// Look for a line containing "okteto-doctor-" and ".zip"
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "okteto-doctor-") && strings.Contains(line, ".zip") {
			// Extract the filename
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.HasPrefix(part, "okteto-doctor-") && strings.HasSuffix(part, ".zip") {
					// If it's just a filename, prepend workdir
					if !filepath.IsAbs(part) && workdir != "" {
						return filepath.Join(workdir, part)
					}
					return part
				}
			}
		}
	}
	return ""
}

// CountFilesInZip counts the number of files in a zip archive
func CountFilesInZip(zipPath string) (int, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open zip file: %w", err)
	}
	defer r.Close()

	return len(r.File), nil
}

// ListFilesInZip returns the list of files in a zip archive
func ListFilesInZip(zipPath string) ([]string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip file: %w", err)
	}
	defer r.Close()

	files := make([]string, 0, len(r.File))
	for _, f := range r.File {
		files = append(files, f.Name)
	}

	return files, nil
}
