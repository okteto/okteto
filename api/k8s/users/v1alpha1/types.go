package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// User represents an okteto user
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Email             string `json:"email,omitempty"`
}

// UserList represents a list of okteto users
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []User `json:"items"`
}
