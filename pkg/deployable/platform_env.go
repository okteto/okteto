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

package deployable

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
)

// envVarNameRegex restricts the env var names the platform can ask us to set
// to a file path. The name is also used as the file name, so it must not be
// able to escape the temporary directory.
var envVarNameRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// GetPlatformEnvironment returns the platform environment (e.g. cloud
// credentials) as a list of "key=value" variables to be passed to the deploy,
// destroy and test commands. Files requested by the platform are written to a
// temporary directory and exposed through their env var. The returned cleanup
// function removes those files and must be called once the commands finish.
// Failures are logged and never block the commands from running.
func GetPlatformEnvironment(ctx context.Context) ([]string, func()) {
	c, err := okteto.NewOktetoClient()
	if err != nil {
		return nil, func() {}
	}
	return getPlatformEnvironment(ctx, c.User(), afero.NewOsFs())
}

func getPlatformEnvironment(ctx context.Context, c types.UserInterface, fs afero.Fs) ([]string, func()) {
	var vars []string

	env, err := c.GetExecutionEnv(ctx)
	if err != nil {
		oktetoLog.Debugf("failed to get platform environment: %s", err)
	}
	for k, v := range env {
		vars = append(vars, fmt.Sprintf("%s=%s", k, v))
		maskValue(v)
	}

	files, err := c.GetExecutionFiles(ctx)
	if err != nil {
		oktetoLog.Debugf("failed to get platform files: %s", err)
	}
	fileVars, cleanup := writePlatformFiles(fs, files)
	vars = append(vars, fileVars...)
	return vars, cleanup
}

// writePlatformFiles writes each file into a private temporary directory and
// returns the "ENV_VAR=path" variables pointing to them.
func writePlatformFiles(fs afero.Fs, files []types.ExecutionFile) ([]string, func()) {
	noop := func() {}
	if len(files) == 0 {
		return nil, noop
	}

	dir, err := afero.TempDir(fs, "", "okteto-platform-")
	if err != nil {
		oktetoLog.Debugf("failed to create platform files dir: %s", err)
		return nil, noop
	}
	cleanup := func() {
		if err := fs.RemoveAll(dir); err != nil {
			oktetoLog.Infof("error removing platform files dir: %s", err)
		}
	}

	var vars []string
	for _, f := range files {
		if !envVarNameRegex.MatchString(f.EnvVar) {
			oktetoLog.Debugf("skipping platform file with invalid env var name %q", f.EnvVar)
			continue
		}
		maskValue(f.Content)
		path := filepath.Join(dir, f.EnvVar)
		if err := afero.WriteFile(fs, path, []byte(f.Content), 0600); err != nil {
			oktetoLog.Debugf("failed to write platform file for %s: %s", f.EnvVar, err)
			continue
		}
		vars = append(vars, fmt.Sprintf("%s=%s", f.EnvVar, path))
	}
	return vars, cleanup
}

// maskValue always masks cloud credentials from the execution environment
func maskValue(v string) {
	if strings.TrimSpace(v) != "" {
		oktetoLog.AddMaskedWord(v)
	}
}
