package test

import (
	"os"
	"time"

	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
)

func getDefaultTimeout() time.Duration {
	defaultTimeout := 5 * time.Minute
	t := os.Getenv(model.OktetoTimeoutEnvVar)
	if t == "" {
		return defaultTimeout
	}

	parsed, err := time.ParseDuration(t)
	if err != nil {
		oktetoLog.Infof("OKTETO_TIMEOUT value is not a valid duration: %s", t)
		oktetoLog.Infof("timeout fallback to defaultTimeout")
		return defaultTimeout
	}

	return parsed
}

func setToSlice[T comparable](set map[T]bool) []T {
	slice := make([]T, 0, len(set))
	for value := range set {
		slice = append(slice, value)
	}
	return slice
}

func setIntersection[T comparable](set1, set2 map[T]bool) map[T]bool {
	intersection := make(map[T]bool)
	for value := range set1 {
		if _, ok := set2[value]; ok {
			intersection[value] = true
		}
	}
	return intersection
}

func sliceToSet[T comparable](slice []T) map[T]bool {
	set := make(map[T]bool)
	for _, value := range slice {
		set[value] = true
	}
	return set
}
