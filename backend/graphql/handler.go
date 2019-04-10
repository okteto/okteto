package graphql

import "github.com/graphql-go/handler"

// Handler returns an http handler for the GraphQL schema
func Handler() *handler.Handler {
	return handler.New(&handler.Config{
		Schema:   &Schema,
		Pretty:   true,
		GraphiQL: true,
	})
}
