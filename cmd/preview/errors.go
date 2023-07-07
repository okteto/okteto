package preview

import "errors"

var errFailedDestroyPreview = errors.New("failed to delete preview")

var errDestroyPreviewTimeout = errors.New("preview destroy didn't finish")

var errFailedSleepPreview = errors.New("failed to sleep preview")

var errFailedWakePreview = errors.New("failed to wake preview")
