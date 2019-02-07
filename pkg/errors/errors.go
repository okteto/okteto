package errors

import (
	"errors"
	"fmt"

	"github.com/cloudnativedevelopment/cnd/pkg/config"
)

var (
	// ErrPodIsGone is raised when we detect a pod shutdown
	ErrPodIsGone = errors.New("pod is gone")

	// ErrLostConnection is raised when we lose network connectivity with the cluster
	ErrLostConnection = fmt.Errorf("Lost connection to your cluster. Please check your network connection and run '%s up' again", config.GetBinaryName())

	// ErrNotDevDeployment is raised when we detect that the deployment was returned to production mode
	ErrNotDevDeployment = errors.New("Deployment is no longer in developer mode")
)
