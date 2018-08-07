package model

import (
	"time"

	"github.com/jinzhu/gorm"
	uuid "github.com/satori/go.uuid"
)

// Model is the base model used to types that are exposed through the API
type Model struct {
	ID        string     `json:"id" yaml:"id" gorm:"primary_key"`
	CreatedAt time.Time  `json:"created,omitempty" yaml:"-"`
	UpdatedAt time.Time  `json:"updated,omitempty" yaml:"-"`
	DeletedAt *time.Time `json:"deleted,omitempty" yaml:"-" sql:"index"`
}

// BeforeCreate adds an ID before creating an object
func (m *Model) BeforeCreate(scope *gorm.Scope) error {
	return scope.SetColumn("ID", uuid.NewV4().String())
}
