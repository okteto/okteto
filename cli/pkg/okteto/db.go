package okteto

import (
	"fmt"
)

// Database is the database and available endpoint
type Database struct {
	Name     string
	Endpoint string
}

// CreateDatabase creates a cloud database
func CreateDatabase(name, space string) (*Database, error) {
	q := ""
	if space == "" {
		q = fmt.Sprintf(`
	  mutation {
			createDatabase(name: "%s") {
		  	name, endpoint
			}
	  }`, name)
	} else {
		q = fmt.Sprintf(`
	  mutation {
			createDatabase(name: "%s", space: "%s") {
		  	name, endpoint
			}
	  }`, name, space)
	}

	var d struct {
		CreateDatabase Database
	}

	if err := query(q, &d); err != nil {
		return nil, fmt.Errorf("error creating database: %s", err)
	}

	return &d.CreateDatabase, nil
}
