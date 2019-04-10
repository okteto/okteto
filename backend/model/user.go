package model

//User represents a user
type User struct {
	ID    string `json:"id,omitempty" yaml:"id,omitempty"`
	Email string `json:"email,omitempty" yaml:"email,omitempty"`
	Name  string `json:"name,omitempty" yaml:"name,omitempty"`
	Token string `json:"token,omitempty" yaml:"token,omitempty"`
}

// NewUser returns a new user with an auth token initialized
func NewUser(id, email, name string) *User {
	return &User{
		ID:    id,
		Email: email,
		Name:  name,
		Token: GenerateRandomString(40),
	}
}
