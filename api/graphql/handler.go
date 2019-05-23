package graphql

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
	"github.com/okteto/app/api/k8s/serviceaccounts"
	"github.com/okteto/app/api/log"
	"github.com/okteto/app/api/model"
)

type key int

const authTokenKey key = 0

// Handler returns an http handler for the GraphQL schema
func Handler() *handler.Handler {

	// Schema holds the GraphQL schema for Okteto
	schema, _ := graphql.NewSchema(
		graphql.SchemaConfig{
			Query:    queryType,
			Mutation: mutationType,
		},
	)

	h := handler.New(&handler.Config{
		Schema:   &schema,
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
			reqToken = strings.TrimSpace(reqToken)
			u, err := validateToken(ctx, reqToken)
			if err == nil {
				ctx = context.WithValue(ctx, authTokenKey, u)
			} else {
				log.Errorf("authentication failure %s", err)
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func validateToken(ctx context.Context, token string) (*model.User, error) {
	u, err := serviceaccounts.GetUserByToken(ctx, token)
	if err != nil {
		log.Errorf("bad token: %s", err)
		return nil, fmt.Errorf("not-authorized")
	}

	return u, nil
}

func getAuthenticatedUser(ctx context.Context) (*model.User, error) {
	u := ctx.Value(authTokenKey)
	us, ok := u.(*model.User)
	if !ok {
		log.Errorf("token stored was not a user: %+v", u)
		return nil, fmt.Errorf("not-authorized")
	}

	return us, nil
}
