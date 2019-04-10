package graphql

import (
	"github.com/okteto/app/backend/model"
)

var devEnvironments = []model.Dev{
	{ID: "1", Name: "dev-env-1", Endpoints: []string{"https://dev-env-1.okteto.dev", "http://dev-env-1.okteto.dev:8000"}},
	{ID: "2", Name: "dev-env-2", Endpoints: []string{"https://dev-env-2.okteto.dev", "http://dev-env-2.okteto.dev:8000"}},
	{ID: "3", Name: "dev-env-3", Endpoints: []string{"https://dev-env-3.okteto.dev", "http://dev-env-3.okteto.dev:8000"}},
}
