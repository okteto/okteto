// Copyright 2025 The Okteto Authors
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

package waitfor

import (
	"errors"
	"strings"

	"github.com/okteto/okteto/pkg/model"
)

var (
	errInvalidService   = errors.New("invalid service format. The service format must be 'kind/service/condition'")
	errInvalidResource  = errors.New("invalid resource type. The resource type must be 'deployment', 'statefulset', or 'job'")
	errInvalidCondition = errors.New("invalid condition. The condition must be 'service_started', 'service_healthy', or 'service_completed_successfully'")
)

const (
	// maxPartsPerService represents the maximum number of parts in a service
	maxPartsPerService = 3

	deploymentResource  = "deployment"
	statefulsetResource = "statefulset"
	jobResource         = "job"
)

// parser represents the parser
type parser struct {
	validateResource  func(string) bool
	validateCondition func(string) bool
}

// parseResult represents the result of the parser
type parseResult struct {
	serviceType string
	serviceName string
	condition   string
}

func newParser() *parser {
	return &parser{
		validateResource: func(resource string) bool {
			return resource == deploymentResource ||
				resource == statefulsetResource ||
				resource == jobResource
		},
		validateCondition: func(condition string) bool {
			return condition == string(model.DependsOnServiceCompleted) ||
				condition == string(model.DependsOnServiceHealthy) ||
				condition == string(model.DependsOnServiceRunning)
		},
	}
}

func (p *parser) parse(service string) (*parseResult, error) {
	substrings := strings.Split(service, "/")
	if len(substrings) != maxPartsPerService {
		return nil, errInvalidService
	}

	serviceType := substrings[0]
	serviceName := substrings[1]
	condition := substrings[2]

	if !p.validateResource(serviceType) {
		return nil, errInvalidResource
	}

	if !p.validateCondition(condition) {
		return nil, errInvalidCondition
	}

	return &parseResult{
		serviceType: serviceType,
		serviceName: serviceName,
		condition:   condition,
	}, nil
}
