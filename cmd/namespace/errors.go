package namespace

import (
	"errors"
)

var errFailedDeleteNamespace = errors.New("failed to delete namespace")

var errDeleteNamespaceTimeout = errors.New("namespace delete didn't finish")
