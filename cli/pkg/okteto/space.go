package okteto

import (
	"fmt"

	"github.com/okteto/app/cli/pkg/errors"
	"github.com/okteto/app/cli/pkg/model"
)

// GetSpaceID returns the space id given its beauty name
func GetSpaceID(name string) (string, error) {
	q := ` query{
		spaces{
			id, name
		},
	}`

	var r struct {
		Spaces []model.Space
	}

	if err := query(q, &r); err != nil {
		return "", fmt.Errorf("error getting spaces: %s", err)
	}

	for _, s := range r.Spaces {
		if s.Name == name {
			return s.ID, nil
		}
		if s.ID == name {
			return s.ID, nil
		}
	}

	return "", errors.ErrNotFound
}
