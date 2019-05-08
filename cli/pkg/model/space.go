package model

//Space represents a dev space
type Space struct {
	ID   string `json:"id,omitempty" yaml:"id,omitempty"`
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}
