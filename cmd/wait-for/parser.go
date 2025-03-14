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
			return resource == "deployment" ||
				resource == "statefulset" ||
				resource == "job"
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
	if len(substrings) != 3 {
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
