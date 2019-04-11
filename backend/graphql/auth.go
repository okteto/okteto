package graphql

import (
	"context"
	"fmt"

	"github.com/okteto/app/backend/k8s/users"
	"github.com/okteto/app/backend/log"
)

func validateToken(ctx context.Context) (string, error) {
	t := ctx.Value(authTokenKey).(string)
	if t == "" {
		return "", fmt.Errorf("bad-request")
	}

	u, err := users.GetByToken(t)
	if err != nil {
		log.Errorf("bad token: %s", err)
		return "", fmt.Errorf("not-authorized")
	}

	return u.ID, nil
}
