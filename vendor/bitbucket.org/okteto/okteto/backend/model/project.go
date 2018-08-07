package model

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/mail"
	"regexp"
	"time"

	yaml "gopkg.in/yaml.v2"
)

// Project represents the highest level object, it contains
// all the deployed applications and the project settings
type Project struct {
	Model
	Name     string `json:"name"`
	DNSName  string `json:"-" gorm:"unique"`
	Settings string `json:"settings,omitempty"`

	// Calculated fields
	LoadedSettings *ProjectSettings `json:"-" gorm:"-"`
	Role           ProjectRole      `gorm:"-" json:"role"`
	PendingUsers   []ProjectACL     `gorm:"-" json:"pendingUsers,omitempty"`
}

//ProjectSettings are the configurations of the projecgt, the users, and the provider information
type ProjectSettings struct {
	Administrators []string  `yaml:"administrators,omitempty"`
	Users          []string  `yaml:"users,omitempty"`
	Provider       *Provider `yaml:"provider,omitempty"`
	Registry       *Registry `yaml:"registry,omitempty"`
	Secrets        []*EnvVar `yaml:"secrets,omitempty"`
}

// ProjectRole represents the type role of a user in a project
type ProjectRole string

//ProjectACL represents which users can access which projects
type ProjectACL struct {
	ProjectID string      `json:"project,omitempty" gorm:"primary_key"`
	UserID    string      `json:"-" gorm:"primary_key"`
	Role      ProjectRole `json:"role,omitempty"`

	CreatedAt time.Time `json:"created,omitempty" yaml:"-"`
	UpdatedAt time.Time `json:"updated,omitempty" yaml:"-"`

	UserEmail string `gorm:"-" json:"user,omitempty"`
}

const (
	//ProjectRoleUser is a normal user of a project
	ProjectRoleUser ProjectRole = "user"

	//ProjectRoleAdmin is an administrator of a project
	ProjectRoleAdmin ProjectRole = "admin"
)

var (
	validProjectName = regexp.MustCompile(`^[a-zA-Z0-9][-a-zA-Z0-9]*$`)
)

//ParseProjectSettings takes s (a base64 encoded string) and returns a ProjectSettings or an error.
func ParseProjectSettings(s string) (*ProjectSettings, *AppError) {
	decodedSettings, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, &AppError{Status: 400, Code: InvalidBase64}
	}

	var settings ProjectSettings
	err = yaml.Unmarshal([]byte(decodedSettings), &settings)

	if err != nil {
		return nil, &AppError{Status: 400, Code: InvalidYAML, Message: err.Error()}
	}

	if len(settings.Administrators) == 0 {
		return nil, &AppError{Status: 400, Code: MissingAdministrators}
	}

	appErr := validateEmails(&settings.Users)
	if appErr != nil {
		return nil, appErr
	}

	appErr = validateEmails(&settings.Administrators)
	if appErr != nil {
		return nil, appErr
	}

	if settings.Provider.Type == "" {
		return nil, &AppError{Status: 400, Code: MissingProviderType}
	}

	if settings.Provider.Type != AWS && settings.Provider.Type != K8 && settings.Provider.Type != Demo {
		return nil, &AppError{Status: 400, Code: InvalidProviderType}
	}

	if settings.Provider.Type == AWS {
		if settings.Provider.AccessKey == "" {
			return nil, &AppError{Status: 400, Code: MissingProviderAccessKey}
		}

		if settings.Provider.SecretKey == "" {
			return nil, &AppError{Status: 400, Code: MissingProviderSecretKey}
		}
	} else if settings.Provider.Type == K8 {
		if settings.Provider.Username == "" {
			return nil, &AppError{Status: 400, Code: InvalidKubernetesConfiguration}
		}
		if settings.Provider.Password == "" {
			return nil, &AppError{Status: 400, Code: InvalidKubernetesConfiguration}
		}
		if settings.Provider.Endpoint == "" {
			return nil, &AppError{Status: 400, Code: InvalidKubernetesConfiguration}
		}
		if settings.Provider.CaCert == "" {
			return nil, &AppError{Status: 400, Code: InvalidKubernetesConfiguration}
		}
	}

	return &settings, nil
}

//Base64EncodeK8Configuration Returns the Base64Encode version of c
func Base64EncodeK8Configuration(c *string) string {
	yamlBytes, err := yaml.Marshal(c)
	if err != nil {
		log.Printf("failed to serialize k8 configuration: %s", err.Error())
		return ""
	}

	return base64.StdEncoding.EncodeToString(yamlBytes)
}

//ToProjectACLs returns a list of ProjectACL based on s.Users
func (p *ProjectSettings) ToProjectACLs(projectID string) []ProjectACL {
	acls := make([]ProjectACL, 0)
	for i := range p.Users {
		acl := ProjectACL{
			ProjectID: projectID,
			UserEmail: p.Users[i],
			Role:      ProjectRoleUser,
		}

		acls = append(acls, acl)
	}

	for i := range p.Administrators {
		acl := ProjectACL{
			ProjectID: projectID,
			UserEmail: p.Administrators[i],
			Role:      ProjectRoleAdmin,
		}

		acls = append(acls, acl)
	}

	return acls
}

//Base64Encode Returns the Base64Encode version of s, as yaml
func (p *ProjectSettings) Base64Encode() string {
	yamlBytes, err := yaml.Marshal(p)
	if err != nil {
		log.Printf("failed to serialize project settings: %s", err.Error())
		return ""
	}

	return base64.StdEncoding.EncodeToString(yamlBytes)
}

// IsDemo returns true is p is a demo provider
func (p *ProjectSettings) IsDemo() bool {
	return p.Provider == nil || p.Provider.Type == "" || p.Provider.Type == Demo
}

//Validate validates that p is valid, and it's not missing any values
func (p *Project) Validate() *AppError {
	if p.Name == "" {
		return &AppError{Status: 400, Code: MissingName}
	}

	if !validProjectName.MatchString(p.Name) {
		return &AppError{Status: 400, Code: InvalidName}
	}

	return nil
}

func validateEmails(emails *[]string) *AppError {
	for _, u := range *emails {
		_, err := mail.ParseAddress(u)
		if err != nil {
			return &AppError{Status: 400, Code: InvalidEmail, Message: fmt.Sprintf("'%s' is an invalid email", u)}
		}
	}

	return nil
}

//IsAdmin returns true if email is an administrator in the project
func (p *ProjectSettings) IsAdmin(email *string) bool {
	for _, e := range p.Administrators {
		if e == *email {
			return true
		}
	}
	return false
}
