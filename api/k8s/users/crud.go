package users

import (
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/okteto/app/api/k8s/users/client"
	"github.com/okteto/app/api/k8s/users/v1alpha1"
	"github.com/okteto/app/api/log"
	"github.com/okteto/app/api/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

var c *client.UserV1Alpha1Client
var namespace string

func getClient() (*client.UserV1Alpha1Client, error) {
	var err error
	if c == nil {
		c, err = client.Get()
		if err != nil {
			return nil, err
		}
		b, err := ioutil.ReadFile(namespaceFile)
		if err != nil {
			return nil, fmt.Errorf("error getting namespace: %s", err)
		}
		namespace = string(b)
	}
	return c, nil
}

// GetByID gets a user by her ID
func GetByID(id string) (*model.User, error) {
	uClient, err := getClient()
	if err != nil {
		return nil, fmt.Errorf("error getting k8s client: %s", err)
	}

	u, err := uClient.Users(namespace).Get(id, metav1.GetOptions{})

	if err != nil {
		return nil, err
	}

	token, ok := u.GetObjectMeta().GetLabels()["token"]
	if !ok {
		return nil, fmt.Errorf("user %s doesn't have a token", id)
	}

	return &model.User{
		ID:    u.Name,
		Token: token,
		Email: u.Email,
	}, nil
}

// GetByToken gets a user by her token
func GetByToken(token string) (*model.User, error) {
	uClient, err := getClient()
	if err != nil {
		return nil, fmt.Errorf("error getting k8s client: %s", err)
	}

	users, err := uClient.Users(namespace).List(
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("token=%s", token),
		},
	)
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

func create(u *model.User) error {
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

// FindOrCreate returns a user or creates it if it doesn't exists
func FindOrCreate(u *model.User) (*model.User, error) {
	found, err := GetByID(u.ID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			err = create(u)

			if err == nil {
				log.Infof("created %s", u.ID)
			}

			return u, err
		}

		return nil, err
	}

	return found, nil
}

// Delete deletes a user
func Delete(user string) error {
	uClient, err := getClient()
	if err != nil {
		return fmt.Errorf("error getting k8s client: %s", err)
	}
	return uClient.Users(namespace).Delete(user, &metav1.DeleteOptions{})
}
