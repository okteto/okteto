package graphql

import (
	"context"
	"fmt"

	"github.com/okteto/app/api/k8s/users"
	"github.com/okteto/app/api/log"
)

func validateToken(ctx context.Context) (string, error) {
	t := ctx.Value(authTokenKey)
	token, ok := t.(string)
	if !ok {
		log.Error("token stored was not a string")
		return "", fmt.Errorf("not-authorized")
	}

	u, err := users.GetByToken(token)
	if err != nil {
		log.Errorf("bad token: %s", err)
		return "", fmt.Errorf("not-authorized")
	}

	return u.ID, nil
}
