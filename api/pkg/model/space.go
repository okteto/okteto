package model

import (
	"fmt"
	"regexp"

	uuid "github.com/satori/go.uuid"
)

var isAlphaNumeric = regexp.MustCompile(`^[a-zA-Z0-9][-a-zA-Z0-9]*$`).MatchString

//Space represents a dev space
type Space struct {
	ID      string   `json:"id,omitempty" yaml:"id,omitempty"`
	Name    string   `json:"name,omitempty" yaml:"name,omitempty"`
	Members []Member `json:"members,omitempty" yaml:"members,omitempty"`
	Invited []Member `json:"invited,omitempty" yaml:"invited,omitempty"`
}

//Member represents a member
type Member struct {
	ID       string `json:"id,omitempty" yaml:"id,omitempty"`
	Name     string `json:"name,omitempty" yaml:"name,omitempty"`
	GithubID string `json:"githubID,omitempty" yaml:"githubID,omitempty"`
	Avatar   string `json:"avatar,omitempty" yaml:"avatar,omitempty"`
	Owner    bool   `json:"owner,omitempty" yaml:"owner,omitempty"`
	Email    string `json:"email,omitempty" yaml:"email,omitempty"`
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

//NewSpace returns a new space
func NewSpace(name string, u *User, members []Member) *Space {
	id := u.ID
	if name != u.GithubID {
		id = uuid.NewV4().String()
	}
	s := &Space{
		ID:   id,
		Name: name,
		Members: []Member{
			Member{
				ID:       u.ID,
				Name:     u.Name,
				GithubID: u.GithubID,
				Avatar:   u.Avatar,
				Owner:    true,
			},
		},
	}
	for _, m := range members {
		s.Members = append(s.Members, m)
	}
	return s
}

//GetSpace returns a space given its name
func GetSpace(u *User) (*Space, error) {
	return &Space{
		ID:   u.ID,
		Name: u.GithubID,
		Members: []Member{
			Member{
				ID:       u.ID,
				Name:     u.Name,
				GithubID: u.GithubID,
				Avatar:   u.Avatar,
				Owner:    true,
			},
		},
	}, nil
}

//GetOwner returns the owner of the namespace
func (s *Space) GetOwner() *Member {
	for _, m := range s.Members {
		if m.Owner {
			return &m
		}
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
