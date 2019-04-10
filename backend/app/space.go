package app

import (
	"fmt"

	"github.com/okteto/app/backend/k8s/client"
	"github.com/okteto/app/backend/k8s/namespaces"
	"github.com/okteto/app/backend/k8s/networkpolicies"
	"github.com/okteto/app/backend/k8s/rolebindings"
	"github.com/okteto/app/backend/k8s/roles"
	"github.com/okteto/app/backend/k8s/serviceaccounts"
	"github.com/okteto/app/backend/model"
)

//CreateSpace configures a namespace for a given space
func CreateSpace(s *model.Space) error {
	c, err := client.Get()
	if err != nil {
		return fmt.Errorf("error getting k8s client: %s", err)
	}

	if err := namespaces.Create(s, c); err != nil {
		return err
	}

	if err := networkpolicies.Create(s, c); err != nil {
		return err
	}

	if err := serviceaccounts.Create(s, c); err != nil {
		return err
	}

	if err := roles.Create(s, c); err != nil {
		return err
	}

	if err := rolebindings.Create(s, c); err != nil {
		return err
	}

	return nil
}
