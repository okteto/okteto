package preview

import "errors"

var errFailedDestroyPreview = errors.New("failed to delete preview")

var errDestroyPreviewTimeout = errors.New("preview destroy didn't finish")
