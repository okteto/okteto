package spaces

import (
	"fmt"

	"github.com/okteto/app/backend/k8s/users/client"
	"github.com/okteto/app/backend/k8s/users/v1alpha1"
	"github.com/okteto/app/backend/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var c *client.UserV1Alpha1Client
var namespace = "pablo"

func getClient() (*client.UserV1Alpha1Client, error) {
	var err error
	if c == nil {
		c, err = client.Get()
		if err != nil {
			return nil, err
		}
	}
	return c, nil
}

// GetByToken gets a user by her token
func GetByToken(token string) (*model.User, error) {
	uClient, err := getClient()
	if err != nil {
		return nil, fmt.Errorf("error getting k8s client: %s", err)
	}

	users, err := uClient.Users(namespace).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("token=%s", token),
	})
	if err != nil {
		return nil, err
	}
	if len(users.Items) == 0 {
		return nil, nil
	}

	if len(users.Items) > 1 {
		return nil, fmt.Errorf("%d users returned for a single token", len(users.Items))
	}

	user := users.Items[0]

	return &model.User{
		ID:    user.Name,
		Token: token,
		Email: user.Email,
	}, nil
}

// Create creates a user
func Create(u *model.User) error {
	uClient, err := getClient()
	if err != nil {
		return fmt.Errorf("error getting k8s client: %s", err)
	}
	uCRD := v1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:   u.ID,
			Labels: map[string]string{"token": u.Token},
		},
		Email: u.Email,
	}
	_, err = uClient.Users(namespace).Create(&uCRD)
	return err
}

// Delete deletes a user
func Delete(user string) error {
	uClient, err := getClient()
	if err != nil {
		return fmt.Errorf("error getting k8s client: %s", err)
	}
	return uClient.Users(namespace).Delete(user, &metav1.DeleteOptions{})
}
