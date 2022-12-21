package externalresource

import (
	"fmt"
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

	uniqueEndpointsNames := make(map[string]bool)
	for _, entry := range result.Endpoints {
		if _, isAdded := uniqueEndpointsNames[entry.Name]; isAdded {
			return fmt.Errorf("there must be no duplicate names for the endpoints of an external resource")
		}

		uniqueEndpointsNames[entry.Name] = false
	}

	er.Icon = result.Icon
	er.Notes.Path = result.Notes

	for _, endpoint := range result.Endpoints {
		er.Endpoints = append(er.Endpoints, ExternalEndpoint(endpoint))
	}

	return nil
}
