package model

import (
	"encoding/base64"
	"log"

	yaml "gopkg.in/yaml.v2"
)

// ParseManifest decodes m and returns a instance of Service
func ParseManifest(m string) (*Service, *AppError) {
	decodedManifest, err := base64.StdEncoding.DecodeString(m)
	if err != nil {
		return nil, &AppError{Status: 400, Code: InvalidBase64}
	}

	var service Service
	err = yaml.Unmarshal([]byte(decodedManifest), &service)

	if err != nil {
		return nil, &AppError{Status: 400, Code: InvalidYAML, Message: err.Error()}
	}

	if service.Name == "" {
		return nil, &AppError{Status: 400, Code: MissingName}
	}

	if !isAlphaNumeric(service.Name) {
		return nil, &AppError{Status: 400, Code: InvalidName}
	}

	if service.Replicas == 0 {
		return nil, &AppError{Status: 400, Code: InvalidReplicaCount}
	}

	return &service, nil
}

//EncodeService return s as a base64 encoded yaml string
func EncodeService(s *Service) string {
	yamlBytes, err := yaml.Marshal(s)
	if err != nil {
		log.Printf("failed to serialize service: %s", err.Error())
		return ""
	}

	return base64.StdEncoding.EncodeToString(yamlBytes)
}
