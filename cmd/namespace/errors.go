package namespace

import (
	"errors"
)

var errFailedDeleteNamespace = errors.New("failed to delete namespace")

var errNoStatusLabel = errors.New("namespace does not have label for status")

var errDeleteNamespaceTimeout = errors.New("namespace delete didn't finish")
