package deploy

import (
	"context"
	"fmt"

	"github.com/okteto/okteto/pkg/okteto"
)

func getOktetoSecretsAsEnvironmenVariables(ctx context.Context) ([]string, error) {
	oktetoClient, err := okteto.NewOktetoClient()
	if err != nil {
		return nil, err
	}
	secrets, err := oktetoClient.GetSecrets(ctx)
	if err != nil {
		return nil, fmt.Errorf("error loading Okteto Secrets: %s", err.Error())
	}

	env := []string{}
	for _, s := range secrets {
		variable := fmt.Sprintf("%s=%s", s.Name, s.Value)
		env = append(env, variable)
	}

	return env, nil
}
