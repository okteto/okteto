package errors

import (
	"errors"
	"fmt"
	"strings"

	"github.com/okteto/okteto/pkg/config"
)

// UserError is meant for errors displayed to the user. It can include a message and a hint
type UserError struct {
	E    error
	Hint string
}

// Error returns the error message
func (u UserError) Error() string {
	return u.E.Error()
}

var (
	// ErrLostConnection is raised when we lose network connectivity with the cluster
	ErrLostConnection = fmt.Errorf("Lost connection to your cluster. Please check your network connection and run '%s up' again", config.GetBinaryName())

	// ErrNotDevDeployment is raised when we detect that the deployment was returned to production mode
	ErrNotDevDeployment = errors.New("Deployment is no longer in developer mode")

	// ErrCommandFailed is raised when the command execution failed
	ErrCommandFailed = errors.New("Command execution failed")

	// ErrNotLogged is raised when we can't get the user token
	ErrNotLogged = fmt.Errorf("please run 'okteto login' and try again")

	// ErrNotFound is raised when an object is not found
	ErrNotFound = fmt.Errorf("not found")

	// ErrInternalServerError is raised when an internal server error or similar is received
	ErrInternalServerError = fmt.Errorf("internal server error, please try again")

	// ErrQuota is returned when there aren't enough resources to enable dev mode
	ErrQuota = fmt.Errorf("Quota exceeded, please free some resources and try again")

	// ErrSyncFrozen is returned when syncthing has been frozen on the same bytes for 30 seconds
	ErrSyncFrozen = fmt.Errorf("The synchronization service hasn't made any progress in the last 2 minutes")
)

// IsNotFound returns true if err is of the type not found
func IsNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
}
