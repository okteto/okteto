package model

import (
	uuid "github.com/satori/go.uuid"
)

// TokenLength is the length of the API token
const TokenLength = 40

//User represents a user
type User struct {
	ID       string
	GithubID string
	Avatar   string
	Name     string
	Email    string
	Token    string
	Invite   string
}

//NewUser returns a new user with an id and auth token initialized
func NewUser(githubID, email, name, avatar string) *User {
	id := uuid.NewV4()
	return &User{
		ID:       id.String(),
		GithubID: githubID,
		Name:     name,
		Email:    email,
		Avatar:   avatar,
		Token:    GenerateRandomString(TokenLength),
	}
}

// GetInvite returns an invite ID
func GetInvite() string {
	return GenerateRandomString(TokenLength)
}

//IsOwner returns if a user is owner
func (u *User) IsOwner(s *Space) bool {
	for _, m := range s.Members {
		if m.ID == u.ID {
			return m.Owner
		}
	}
	return false
}
