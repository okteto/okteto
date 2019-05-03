package app

import (
	"fmt"

	"github.com/okteto/app/api/k8s/client"
	"github.com/okteto/app/api/k8s/deployments"
	"github.com/okteto/app/api/k8s/namespaces"
	"github.com/okteto/app/api/k8s/networkpolicies"
	"github.com/okteto/app/api/k8s/rolebindings"
	"github.com/okteto/app/api/k8s/roles"
	"github.com/okteto/app/api/k8s/serviceaccounts"
	"github.com/okteto/app/api/k8s/statefulsets"
	"github.com/okteto/app/api/model"
)

//CreateSpace configures a namespace for a given user
func CreateSpace(u *model.User) (*model.Space, error) {
	c, err := client.Get()
	if err != nil {
		return nil, fmt.Errorf("error getting k8s client: %s", err)
	}

	s := &model.Space{
		ID:   u.ID,
		Name: u.GithubID,
	}

	if err := namespaces.Create(s, c); err != nil {
		return nil, err
	}

	if err := networkpolicies.Create(s, c); err != nil {
		return nil, err
	}

	if err := serviceaccounts.Create(s, c); err != nil {
		return nil, err
	}

	if err := roles.Create(s, c); err != nil {
		return nil, err
	}

	if err := rolebindings.Create(s, c); err != nil {
		return nil, err
	}

	return s, nil
}

//GetCredential returns the credentials of the user for her space
func GetCredential(u *model.User) (string, error) {
	s := &model.Space{
		ID:   u.ID,
		Name: u.GithubID,
	}

	credential, err := serviceaccounts.GetCredentialConfig(s)
	if err != nil {
		return "", err
	}

	return credential, err
}

//ListDevEnvs returns the dev environments for a given user
func ListDevEnvs(u *model.User) ([]*model.Dev, error) {
	s := &model.Space{
		ID:   u.ID,
		Name: u.GithubID,
	}

	c, err := client.Get()
	if err != nil {
		return nil, fmt.Errorf("error getting k8s client: %s", err)
	}

	deploys, err := deployments.List(s, c)
	if err != nil {
		return nil, fmt.Errorf("error getting deployments: %s", err)
	}

	result := []*model.Dev{}
	for _, d := range deploys {
		dev := &model.Dev{
			ID:   d.Name,
			Name: d.Name,
		}
		if deployments.IsDevModeOn(&d) {
			dev.Dev = &model.Member{
				ID:       u.ID,
				Name:     u.Name,
				GithubID: u.GithubID,
				Avatar:   u.Avatar,
				Owner:    true,
			}
		}
		dev.Endpoints = BuildEndpoints(u, dev)
		result = append(result, dev)
	}
	return result, nil
}

//ListSpaces returns the spaces for a given user
func ListSpaces(u *model.User) ([]*model.Space, error) {
	s := &model.Space{
		ID:   u.ID,
		Name: u.GithubID,
		Members: []model.Member{
			model.Member{
				ID:       u.ID,
				Name:     u.Name,
				GithubID: u.GithubID,
				Avatar:   u.Avatar,
				Owner:    true,
			},
		},
	}
	return []*model.Space{s}, nil
}

//ListDatabases returns the databases for a given user
func ListDatabases(u *model.User) ([]*model.DB, error) {
	s := &model.Space{
		ID:   u.ID,
		Name: u.GithubID,
	}
	c, err := client.Get()
	if err != nil {
		return nil, fmt.Errorf("error getting k8s client: %s", err)
	}

	sfss, err := statefulsets.List(s, c)
	if err != nil {
		return nil, fmt.Errorf("error getting statefulsets: %s", err)
	}
	result := []*model.DB{}
	for _, sfs := range sfss {
		db := &model.DB{
			ID:   sfs.Name,
			Name: sfs.Name,
		}
		if db.Name == model.POSTGRES {
			for _, c := range sfs.Spec.Template.Spec.Containers {
				if c.Name == model.POSTGRES {
					for _, e := range c.Env {
						if e.Name == "POSTGRES_PASSWORD" {
							db.Password = e.Value
							break
						}
					}
					break
				}
			}
		}
		db.Endpoint = db.GetEndpoint()
		result = append(result, db)
	}

	return result, nil
}

// BuildEndpoints builds the endpoints with the FQDN for the specific user and dev env
func BuildEndpoints(u *model.User, d *model.Dev) []string {
	s := &model.Space{
		ID:   u.ID,
		Name: u.GithubID,
	}

	return []string{fmt.Sprintf("https://%s", d.GetEndpoint(s))}
}
