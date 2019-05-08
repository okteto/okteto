package graphql

import (
	"context"
	"fmt"

	"github.com/okteto/app/api/k8s/serviceaccounts"
	"github.com/okteto/app/api/log"
	"github.com/okteto/app/api/model"
)

func validateToken(ctx context.Context) (*model.User, error) {
	t := ctx.Value(authTokenKey)
	token, ok := t.(string)
	if !ok {
		log.Error("token stored was not a string")
		return nil, fmt.Errorf("not-authorized")
	}

	u, err := serviceaccounts.GetUserByToken(token)
	if err != nil {
		log.Errorf("bad token: %s", err)
		return nil, fmt.Errorf("not-authorized")
	}

	return u, nil
}
