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
func CreateSpace(s *model.Space) error {
	c, err := client.Get()
	if err != nil {
		return fmt.Errorf("error getting k8s client: %s", err)
	}

	if err := namespaces.Create(s, c); err != nil {
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

//ExistsByName returns if a space exists for a name
func ExistsByName(name, owner string) bool {
	c, err := client.Get()
	if err != nil {
		return true
	}

	olds, err := namespaces.GetByLabel(fmt.Sprintf("%s=%s, %s=%s", namespaces.OktetoNameLabel, name, namespaces.OktetoOwnerLabel, owner), c)
	if err != nil {
		return true
	}
	if len(olds) > 0 {
		return true
	}
	return false
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
			if owner.ID != s.ID {
				s.Name = fmt.Sprintf("%s@%s", s.Name, owner.GithubID)
			}
		}
		spaces = append(spaces, s)
	}
	return spaces, nil
}
