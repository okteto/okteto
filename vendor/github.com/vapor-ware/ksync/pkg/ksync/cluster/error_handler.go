package cluster

import (
	log "github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/util/runtime"
)

func logErrorHandler(err error) {
	log.Debug(err)
}

// SetErrorHandlers modifies the default runtime handlers to replace the default
// logger with our own.
func SetErrorHandlers() {
	// This is a massive, unfortunate hack that assumes the default log handler is
	// the first one.
	runtime.ErrorHandlers[0] = logErrorHandler
}
