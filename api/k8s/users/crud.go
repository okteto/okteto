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
var errNotFound = fmt.Errorf("not found")

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

// GetByGithubID gets a user by her githubID
func GetByGithubID(githubID string) (*model.User, error) {
	log.Info("finding user by her githubID")
	uClient, err := getClient()
	if err != nil {
		return nil, fmt.Errorf("error getting k8s client: %s", err)
	}

	users, err := uClient.Users(namespace).List(
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("github=%s", githubID),
		},
	)
	if err != nil {
		return nil, err
	}
	if len(users.Items) == 0 {
		log.Info("user not found by her githubID")
		return nil, errNotFound
	}

	if len(users.Items) > 1 {
		return nil, fmt.Errorf("%d users returned for a single githubID", len(users.Items))
	}

	u := users.Items[0]
	log.Info("found user by her githubID")
	return v1alpha1.ToModel(&u), nil
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
		return nil, errNotFound
	}

	if len(users.Items) > 1 {
		return nil, fmt.Errorf("%d users returned for a single token", len(users.Items))
	}

	u := users.Items[0]
	return v1alpha1.ToModel(&u), nil
}

func create(u *model.User) error {
	uClient, err := getClient()
	if err != nil {
		return fmt.Errorf("error getting k8s client: %s", err)
	}

	uCRD := v1alpha1.NewUser(u)
	_, err = uClient.Users(namespace).Create(uCRD)
	return err
}

// FindOrCreate returns a user or creates it if it doesn't exists
func FindOrCreate(u *model.User) (*model.User, error) {
	found, err := GetByGithubID(u.GithubID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Info("user not found, creating")
			err = create(u)

			if err == nil {
				log.Infof("created %s for %s", u.ID, u.GithubID)
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
