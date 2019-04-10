package graphql

import "github.com/graphql-go/graphql"

var devEnvironmentType = graphql.NewObject(
	graphql.ObjectConfig{
		Name: "DevEnvironment",
		Fields: graphql.Fields{
			"id": &graphql.Field{
				Type: graphql.String,
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
		},
	})

var Schema, _ = graphql.NewSchema(
	graphql.SchemaConfig{
		Query: queryType,
	},
)
