package errors

import "fmt"

// ErrNotFound is raised when an object is not found
var ErrNotFound = fmt.Errorf("not found")
