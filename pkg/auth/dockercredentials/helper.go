package dockercredentials

import (
	"errors"

	"github.com/docker/docker-credential-helpers/credentials"
)

var ErrNotImplemented = errors.New("not implemented")

type RegistryCredentialsGetter interface {
	GetRegistryCredentials(host string) (string, string, error)
}

type OktetoClusterHelper struct {
	getter RegistryCredentialsGetter
}

var _ credentials.Helper = (*OktetoClusterHelper)(nil)

func NewOktetoClusterHelper(getter RegistryCredentialsGetter) *OktetoClusterHelper {
	return &OktetoClusterHelper{getter: getter}
}

// Add appends credentials to the store.
func (och *OktetoClusterHelper) Add(*credentials.Credentials) error {
	return ErrNotImplemented
}

// Delete removes credentials from the store.
func (och *OktetoClusterHelper) Delete(serverURL string) error {
	return ErrNotImplemented
}

// Get retrieves credentials from the store.
// It returns username and secret as strings.
func (och *OktetoClusterHelper) Get(serverURL string) (string, string, error) {
	return och.getter.GetRegistryCredentials(serverURL)
}

// List returns the stored serverURLs and their associated usernames.
func (och *OktetoClusterHelper) List() (map[string]string, error) {
	return nil, ErrNotImplemented
}
