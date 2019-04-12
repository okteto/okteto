package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Space represents an space
type Space struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// SpaceList represents a list of spaces
type SpaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Space `json:"items"`
}
