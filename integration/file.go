// Copyright 2022 The Okteto Authors
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

package integration

import (
	"os"
	"text/template"
)

type deployment struct {
	Name string
}

func WriteDeployment(template *template.Template, name, path string) error {
	dFile, err := os.Create(path)
	if err != nil {
		return err
	}

	if err := template.Execute(dFile, deployment{Name: name}); err != nil {
		return err
	}
	defer dFile.Close()
	return nil
}
