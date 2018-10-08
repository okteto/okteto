package debug

import (
	"github.com/imdario/mergo"
	log "github.com/sirupsen/logrus"
)

// MergeFields takes a slice of logging fields and merges them together.
func MergeFields(fieldSlice ...log.Fields) log.Fields {
	fields := &log.Fields{}
	for _, src := range fieldSlice {
		if err := mergo.Merge(fields, src); err != nil {
			return nil
		}
	}

	return *fields
}
