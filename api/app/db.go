package app

import (
	"fmt"

	"github.com/okteto/app/api/k8s/client"
	"github.com/okteto/app/api/k8s/services"
	"github.com/okteto/app/api/k8s/statefulsets"
	"github.com/okteto/app/api/k8s/statefulsets/mongo"
	"github.com/okteto/app/api/k8s/statefulsets/postgres"
	"github.com/okteto/app/api/k8s/statefulsets/redis"
	"github.com/okteto/app/api/model"
	apiv1 "k8s.io/api/core/v1"
)

//CreateDatabase creates a database
func CreateDatabase(db *model.DB, s *model.Space) error {
	c, err := client.Get()
	if err != nil {
		return fmt.Errorf("error getting k8s client: %s", err)
	}

	if err := statefulsets.Deploy(db, s, c); err != nil {
		return err
	}

	var dbService *apiv1.Service
	switch db.Name {
	case model.MONGO:
		dbService = mongo.TranslateService(s)
	case model.REDIS:
		dbService = redis.TranslateService(s)
	case model.POSTGRES:
		dbService = postgres.TranslateService(s)
	default:
		return fmt.Errorf("Supported databases are: mongo, redis or postgres")
	}
	if err := services.Deploy(dbService, s, c); err != nil {
		return err
	}
	return nil
}

//DestroyDatabase destroys a database
func DestroyDatabase(db *model.DB, s *model.Space) error {
	c, err := client.Get()
	if err != nil {
		return fmt.Errorf("error getting k8s client: %s", err)
	}

	if err := services.Destroy(db.Name, s, c); err != nil {
		return err
	}

	if err := statefulsets.Destroy(db, s, c); err != nil {
		return err
	}

	return nil
}
