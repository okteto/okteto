package spaces

import (
	"fmt"

	"github.com/okteto/app/backend/k8s/spaces/client"
	"github.com/okteto/app/backend/k8s/spaces/v1alpha1"
	"github.com/okteto/app/backend/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var c *client.SpaceV1Alpha1Client
var namespace = "pablo"

func getClient() (*client.SpaceV1Alpha1Client, error) {
	var err error
	if c == nil {
		c, err = client.Get()
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

// List gets the spaces for a given user
func List(user string) ([]*model.Space, error) {
	result := []*model.Space{}
	sClient, err := getClient()
	if err != nil {
		return nil, fmt.Errorf("error getting k8s client: %s", err)
	}

	spaces, err := sClient.Spaces(namespace).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("user-%s=true", user),
	})
	if err != nil {
		return nil, err
	}

	for _, s := range spaces.Items {
		result = append(
			result,
			&model.Space{
				Name:    s.Name,
				Members: []string{user},
			},
		)
	}
	return result, nil
}

// Create creates a space
func Create(s *model.Space) error {
	sClient, err := getClient()
	if err != nil {
		return fmt.Errorf("error getting k8s client: %s", err)
	}

	labels := map[string]string{}
	for _, member := range s.Members {
		labels[fmt.Sprintf("user-%s", member)] = "true"
	}
	sCRD := v1alpha1.Space{
		ObjectMeta: metav1.ObjectMeta{
			Name:   s.Name,
			Labels: labels,
		},
	}

	_, err = sClient.Spaces(namespace).Create(&sCRD)
	return err
}

// Delete deletes a space
func Delete(s *model.Space) error {
	sClient, err := getClient()
	if err != nil {
		return fmt.Errorf("error getting k8s client: %s", err)
	}
	return sClient.Spaces(namespace).Delete(s.Name, &metav1.DeleteOptions{})
}
