package externalresource

import (
	"fmt"
	"strings"
)

const (
	defValueForIcon = "default"
)

var (
	possibleIconValues = map[string]bool{
		"container":     true,
		"dashboard":     true,
		"database":      true,
		defValueForIcon: true,
		"function":      true,
		"graph":         true,
		"storage":       true,
	}
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

	if result.Icon == "" {
		result.Icon = defValueForIcon
	} else if _, ok := possibleIconValues[result.Icon]; !ok {
		keys := make([]string, 0, len(possibleIconValues))
		for k := range possibleIconValues {
			keys = append(keys, fmt.Sprintf("'%s'", k))
		}
		return fmt.Errorf("'%s' is not supported as icon value. Supported values: %s", result.Icon, strings.Join(keys, ", "))
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
		er.Endpoints = append(er.Endpoints, ExternalEndpoint(endpoint))
	}

	return nil
}
