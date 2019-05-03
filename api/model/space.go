package model

import (
	"fmt"
	"regexp"
)

var isAlphaNumeric = regexp.MustCompile(`^[a-zA-Z0-9][-a-zA-Z0-9]*$`).MatchString

//Space represents a dev space
type Space struct {
	ID      string   `json:"id,omitempty" yaml:"id,omitempty"`
	Name    string   `json:"name,omitempty" yaml:"name,omitempty"`
	Members []string `json:"members,omitempty" yaml:"members,omitempty"`
}

//Member represents a member
type Member struct {
	ID       string `json:"id,omitempty" yaml:"id,omitempty"`
	Name     string `json:"name,omitempty" yaml:"name,omitempty"`
	GithubID string `json:"githubID,omitempty" yaml:"githubID,omitempty"`
	Avatar   string `json:"avatar,omitempty" yaml:"avatar,omitempty"`
	Owner    bool   `json:"owner,omitempty" yaml:"owner,omitempty"`
}

func (s *Space) validate() error {
	if s.ID == "" {
		return fmt.Errorf("Name cannot be empty")
	}

	if !isAlphaNumeric(s.ID) {
		return fmt.Errorf("Name must be alphanumeric")
	}

	return nil
}

//GetNetworkPolicyName returns the network policy name for a namespace
func (s *Space) GetNetworkPolicyName() string {
	return s.ID
}

//GetDNSPolicyName returns the dns policy name for a namespace
func (s *Space) GetDNSPolicyName() string {
	return fmt.Sprintf("dns-%s", s.ID)
}
