package app

import (
	"fmt"

	"github.com/okteto/app/api/pkg/k8s/client"
	"github.com/okteto/app/api/pkg/k8s/services"
	"github.com/okteto/app/api/pkg/k8s/statefulsets"
	"github.com/okteto/app/api/pkg/k8s/statefulsets/mongo"
	"github.com/okteto/app/api/pkg/k8s/statefulsets/postgres"
	"github.com/okteto/app/api/pkg/k8s/statefulsets/redis"
	"github.com/okteto/app/api/pkg/k8s/volumes"
	"github.com/okteto/app/api/pkg/model"
	apiv1 "k8s.io/api/core/v1"
)

//CreateDatabase creates a database
func CreateDatabase(db *model.DB, s *model.Space) error {
	c := client.Get()
	if err := volumes.Create(db.GetVolumeName(), s, c); err != nil {
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

	if err := statefulsets.Deploy(db, s, c); err != nil {
		return err
	}

	if err := services.Deploy(dbService, s, c); err != nil {
		return err
	}
	return nil
}

//DestroyDatabase destroys a database
func DestroyDatabase(db *model.DB, s *model.Space) error {
	c := client.Get()

	if err := services.Destroy(db.Name, s, c); err != nil {
		return err
	}

	if err := statefulsets.Destroy(db, s, c); err != nil {
		return err
	}

	if err := volumes.Destroy(db.GetVolumeName(), s, c); err != nil {
		return err
	}

	return nil
}

//ListDatabases returns the databases for a given user
func ListDatabases(s *model.Space) ([]*model.DB, error) {
	c := client.Get()
	sfss, err := statefulsets.List(s, c)
	if err != nil {
		return nil, fmt.Errorf("error getting statefulsets: %s", err)
	}
	result := []*model.DB{}
	for _, sfs := range sfss {
		db := &model.DB{
			ID:    sfs.Name,
			Space: s.ID,
			Name:  sfs.Name,
		}
		if db.Name == model.POSTGRES {
			for _, c := range sfs.Spec.Template.Spec.Containers {
				if c.Name == model.POSTGRES {
					for _, e := range c.Env {
						if e.Name == "POSTGRES_PASSWORD" {
							db.Password = e.Value
							break
						}
					}
					break
				}
			}
		}
		db.Endpoint = db.GetEndpoint()
		result = append(result, db)
	}

	return result, nil
}
