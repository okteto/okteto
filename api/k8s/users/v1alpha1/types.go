package v1alpha1

import (
	"github.com/okteto/app/api/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// User represents an okteto user
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Avatar            string `json:"avatar,omitempty"`
	Email             string `json:"email,omitempty"`
	FullName          string `json:"fullname,omitempty"`
}

// UserList represents a list of okteto users
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []User `json:"items"`
}

// GetGithubID returns the githubID of u
func (u *User) GetGithubID() string {
	return u.GetObjectMeta().GetLabels()["github"]
}

// GetToken returns the token of u
func (u *User) GetToken() string {
	return u.GetObjectMeta().GetLabels()["token"]
}

// NewUser returns a new User with the information from u
func NewUser(u *model.User) *User {
	return &User{
		ObjectMeta: metav1.ObjectMeta{
			Name: u.ID,
			Labels: map[string]string{
				"token":  u.Token,
				"github": u.GithubID},
		},
		Email:    u.Email,
		FullName: u.Name,
		Avatar:   u.Avatar,
	}
}

// ToModel converts u into a model.User
func ToModel(u *User) *model.User {
	return &model.User{
		ID:       u.Name,
		GithubID: u.GetGithubID(),
		Token:    u.GetToken(),
		Email:    u.Email,
		Name:     u.FullName,
		Avatar:   u.Avatar,
	}
}
