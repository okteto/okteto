package model

import (
	"regexp"
)

var isAlphaNumeric = regexp.MustCompile(`^[a-zA-Z0-9][-a-zA-Z0-9]*$`).MatchString

//Space represents a dev space
type Space struct {
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	User string `json:"user,omitempty" yaml:"user,omitempty"`
}
