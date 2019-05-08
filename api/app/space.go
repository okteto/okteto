package app

import (
	"fmt"

	"github.com/okteto/app/api/k8s/client"
	"github.com/okteto/app/api/k8s/namespaces"
	"github.com/okteto/app/api/k8s/networkpolicies"
	"github.com/okteto/app/api/k8s/rolebindings"
	"github.com/okteto/app/api/k8s/roles"
	"github.com/okteto/app/api/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//CreateSpace configures a namespace for a given user
func CreateSpace(s *model.Space, upsert bool) error {
	c, err := client.Get()
	if err != nil {
		return fmt.Errorf("error getting k8s client: %s", err)
	}

	if err := namespaces.Create(s, c, upsert); err != nil {
		return err
	}

	if err := roles.Create(s, c); err != nil {
		return err
	}

	if err := rolebindings.Create(s, c); err != nil {
		return err
	}

	if err := networkpolicies.Create(s, c); err != nil {
		return err
	}

	return nil
}

//DeleteSpace deletes a namespace for a given user
func DeleteSpace(s *model.Space) error {
	c, err := client.Get()
	if err != nil {
		return fmt.Errorf("error getting k8s client: %s", err)
	}
	if err := namespaces.Destroy(s, c); err != nil {
		return err
	}
	return nil
}

//ListSpaces returns the spaces for a given user
func ListSpaces(u *model.User) ([]*model.Space, error) {
	c, err := client.Get()
	if err != nil {
		return nil, fmt.Errorf("error getting k8s client: %s", err)
	}

	ns, err := c.CoreV1().Namespaces().List(
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=true", fmt.Sprintf(namespaces.OktetoMemberLabelTemplate, u.ID)),
		},
	)
	if err != nil {
		return nil, err
	}
	spaces := []*model.Space{}
	for _, n := range ns.Items {
		s := namespaces.ToModel(&n)
		if !u.IsOwner(s) {
			owner := s.GetOwner()
			s.Name = fmt.Sprintf("%s@%s", s.Name, owner.GithubID)
		}
		spaces = append(spaces, s)
	}
	return spaces, nil
}
