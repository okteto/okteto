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

package externalresource

import (
	"fmt"

	"github.com/a8m/envsubst"
)

type externalResourceUnmarshaller struct {
	Icon      string                         `yaml:"icon,omitempty"`
	Notes     string                         `yaml:"notes,omitempty"`
	Endpoints []externalEndpointUnmarshaller `yaml:"endpoints,omitempty"`
}

type externalEndpointUnmarshaller struct {
	Name string `yaml:"name,omitempty"`
	Url  string `yaml:"url,omitempty"`
}

func (er *ExternalResource) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var result externalResourceUnmarshaller
	err := unmarshal(&result)
	if err != nil {
		return err
	}

	if len(result.Endpoints) < 1 {
		return fmt.Errorf("there must be at least one endpoint available for the external resource")
	}

	er.Icon = result.Icon

	uniqueEndpointsNames := make(map[string]bool)
	for _, entry := range result.Endpoints {
		if _, isAdded := uniqueEndpointsNames[entry.Name]; isAdded {
			return fmt.Errorf("there must be no duplicate names for the endpoints of an external resource")
		}

		uniqueEndpointsNames[entry.Name] = false
	}

	if result.Notes != "" {
		er.Notes = &Notes{
			Path: result.Notes,
		}
	}

	for _, endpoint := range result.Endpoints {
		name, err := envsubst.String(endpoint.Name)
		if err != nil {
			return fmt.Errorf("error expanding environment on '%s': %w", endpoint.Name, err)
		}
		url, err := envsubst.String(endpoint.Url)
		if err != nil {
			return fmt.Errorf("error expanding environment on '%s': %w", endpoint.Name, err)
		}
		er.Endpoints = append(er.Endpoints, &ExternalEndpoint{
			Name: name,
			Url:  url,
		})
	}

	return nil
}
