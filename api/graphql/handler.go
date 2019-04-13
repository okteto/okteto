package graphql

import (
	"context"
	"net/http"
	"strings"

	"github.com/graphql-go/handler"
)

type key int

const authTokenKey key = 0

// Handler returns an http handler for the GraphQL schema
func Handler() *handler.Handler {
	h := handler.New(&handler.Config{
		Schema:   &Schema,
		Pretty:   true,
		GraphiQL: false,
	})

	return h
}

// TokenMiddleware is a middleware that extracts the bearer token and puts it in the context
func TokenMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		h := r.Header.Get("Authorization")
		splitToken := strings.Split(h, "Bearer")

		if len(splitToken) > 1 {
			reqToken := splitToken[1]
			ctx = context.WithValue(ctx, authTokenKey, reqToken)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
