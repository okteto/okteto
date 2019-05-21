package serviceaccounts

import (
	"context"
	"fmt"
	"strings"

	"github.com/okteto/app/api/k8s/client"
	"github.com/okteto/app/api/k8s/secrets"
	"github.com/okteto/app/api/log"
	"github.com/okteto/app/api/model"
	"github.com/opentracing/opentracing-go"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Create creates a service account for a given space
func Create(u *model.User, c *kubernetes.Clientset) error {
	log.Debugf("Creating service account '%s'...", u.ID)
	old, err := c.CoreV1().ServiceAccounts(client.GetOktetoNamespace()).Get(u.ID, metav1.GetOptions{})
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("Error getting kubernetes service account: %s", err)
	}
	sa := translate(u)
	if old.Name == "" {
		_, err = c.CoreV1().ServiceAccounts(client.GetOktetoNamespace()).Create(sa)
		if err != nil {
			return fmt.Errorf("Error creating kubernetes service account: %s", err)
		}
		log.Debugf("Created service account '%s'.", u.ID)
	} else {
		_, err = c.CoreV1().ServiceAccounts(client.GetOktetoNamespace()).Update(sa)
		if err != nil {
			return fmt.Errorf("Error updating kubernetes service account: %s", err)
		}
		log.Debugf("Updated service account '%s'.", u.ID)
	}
	return nil
}

//GetCredentialConfig returns the credential for accessing the dev mode container
func GetCredentialConfig(u *model.User, space string) (string, error) {
	log.Debug("Get service account credential")
	c := client.Get()
	sa, err := c.CoreV1().ServiceAccounts(client.GetOktetoNamespace()).Get(u.ID, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("Error getting kubernetes service account: %s", err)
	}
	secret, err := secrets.Get(sa.Secrets[0].Name, client.GetOktetoNamespace(), c)
	if err != nil {
		return "", err
	}
	return getConfigB64(space, string(secret.Data["ca.crt"]), string(secret.Data["token"])), nil
}

// GetUserByToken gets a user by her token
func GetUserByToken(ctx context.Context, token string) (*model.User, error) {
	if len(token) == 0 {
		return nil, fmt.Errorf("empty token")
	}
	if len(token) > model.TokenLength {
		return nil, fmt.Errorf("malformed token, too long")
	}

	c := client.Get()
	sa, err := getByLabel(ctx, fmt.Sprintf("%s=%s", OktetoTokenLabel, token), c)
	if err != nil {
		return nil, err
	}
	return ToModel(sa), nil
}

//GetUserByGithubID returns a user by githubID
func GetUserByGithubID(ctx context.Context, githubID string) (*model.User, error) {
	log.Debug("finding user by her githubID")
	c := client.Get()

	sa, err := getByLabel(ctx, fmt.Sprintf("%s=%s", OktetoGithubIDLabel, githubID), c)
	if err != nil {
		return nil, err
	}
	log.Debug("found user by her githubID")
	return ToModel(sa), nil
}

// GetUserByID gets a user by her id
func GetUserByID(ctx context.Context, id string) (*model.User, error) {
	c := client.Get()

	sa, err := getByLabel(ctx, fmt.Sprintf("%s=%s", OktetoIDLabel, id), c)
	if err != nil {
		return nil, err
	}
	return ToModel(sa), nil
}

func getByLabel(ctx context.Context, label string, c *kubernetes.Clientset) (*apiv1.ServiceAccount, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "k8s.serviceaccounts.crud.getbylabel")
	defer span.Finish()

	sas, err := c.CoreV1().ServiceAccounts(client.GetOktetoNamespace()).List(
		metav1.ListOptions{
			LabelSelector: label,
		},
	)
	if err != nil {
		return nil, err
	}
	if len(sas.Items) == 0 {
		return nil, fmt.Errorf("not found")
	}
	return &sas.Items[0], nil
}

// ToModel converts a service account into a model.User
func ToModel(sa *apiv1.ServiceAccount) *model.User {
	return &model.User{
		ID:       sa.Labels[OktetoIDLabel],
		GithubID: sa.Labels[OktetoGithubIDLabel],
		Token:    sa.Labels[OktetoTokenLabel],
		Email:    sa.Annotations[OktetoEmailAnnotation],
		Name:     sa.Annotations[OktetoNameAnnotation],
		Avatar:   sa.Annotations[OktetoAvatarAnnotation],
	}
}
