package errors

import (
	"errors"
	"fmt"

	"github.com/okteto/app/cli/pkg/config"
)

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
)
