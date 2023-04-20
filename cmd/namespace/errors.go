package namespace

import (
	"errors"
)

var errFailedDeleteNamespace = errors.New("failed to delete namespace")

var errDeleteNamespaceTimeout = errors.New("namespace delete didn't finish")

var errFailedSleepNamespace = errors.New("failed to sleep namespace")

var errFailedWakeNamespace = errors.New("failed to wake namespace")
