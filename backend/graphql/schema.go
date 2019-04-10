package graphql

import (
	"fmt"

	"github.com/graphql-go/graphql"
	"github.com/okteto/app/backend/app"
	"github.com/okteto/app/backend/github"
	"github.com/okteto/app/backend/log"
	"github.com/okteto/app/backend/model"
)

var devEnvironmentType = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "DevEnvironment",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.ID,
			},
			"name": &graphql.Field{
				Type: graphql.String,
			},
			"endpoints": &graphql.Field{
				Type: graphql.NewList(graphql.String),
			},
		},
	},
)

var userType = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "User",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.ID,
			},
			"email": &graphql.Field{
				Type: graphql.String,
			},
			"name": &graphql.Field{
				Type: graphql.String,
			},
			"githubUserId": &graphql.Field{
				Type: graphql.String,
			},
			"createdAt": &graphql.Field{
				Type: graphql.DateTime,
			},
			"updatedAt": &graphql.Field{
				Type: graphql.DateTime,
			},
		},
	},
)

var authenticatedUserType = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "me",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.ID,
			},
			"email": &graphql.Field{
				Type: graphql.String,
			},
			"name": &graphql.Field{
				Type: graphql.String,
			},
			"token": &graphql.Field{
				Type: graphql.String,
			},
		},
	},
)

var queryType = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"environments": &graphql.Field{
				Type:        graphql.NewList(devEnvironmentType),
				Description: "Get environment list",
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {
					return devEnvironments, nil
				},
			},
			"environmentByID": &graphql.Field{
				Type:        devEnvironmentType,
				Description: "Get environment by ID",
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.ID),
					},
				},
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {
					id := params.Args["id"].(string)
					for _, d := range devEnvironments {
						if d.ID == id {
							return d, nil
						}
					}
					return nil, fmt.Errorf("%s not found", id)
				},
			},
		},
	})

var mutationType = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"auth": &graphql.Field{
				Type:        authenticatedUserType,
				Description: "Authenticate a user with github",
				Args: graphql.FieldConfigArgument{
					"code": &graphql.ArgumentConfig{
						Type: graphql.NewNonNull(graphql.String),
					},
				},
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {

					code := params.Args["code"].(string)
					u, err := github.Auth(code)
					if err != nil {
						log.Errorf("failed to auth user: %s", err)
						return nil, fmt.Errorf("failed to authenticate")
					}

					s := &model.Space{Name: u.ID, Members: []string{u.ID}}
					if err := app.CreateSpace(s); err != nil {
						log.Errorf("failed to create space for %s: %s", s.Name, err)
						return nil, fmt.Errorf("failed to create your space")
					}

					log.Infof("space created for %s", s.Name)

					return u, nil
				},
			},
		},
	},
)

// Schema holds the GraphQL schema for Okteto
var Schema, _ = graphql.NewSchema(
	graphql.SchemaConfig{
		Query:    queryType,
		Mutation: mutationType,
	},
)
