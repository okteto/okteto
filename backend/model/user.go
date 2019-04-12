package model

//User represents a user
type User struct {
	ID    string `json:"id,omitempty" yaml:"id,omitempty"`
	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	Email string `json:"email,omitempty" yaml:"email,omitempty"`
	Token string `json:"token,omitempty" yaml:"token,omitempty"`
}

// NewUser returns a new user with an auth token initialized
func NewUser(id, email, name string) *User {
	return &User{
		ID:    id,
		Name:  name,
		Email: email,
		Token: GenerateRandomString(40),
	}
}
