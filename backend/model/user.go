package model

//User represents a user
type User struct {
	ID    string `json:"id,omitempty" yaml:"id,omitempty"`
	Token string `json:"token,omitempty" yaml:"token,omitempty"`
}
