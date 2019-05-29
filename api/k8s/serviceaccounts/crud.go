package serviceaccounts

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/okteto/app/api/k8s/client"
	"github.com/okteto/app/api/k8s/secrets"
	"github.com/okteto/app/api/log"
	"github.com/okteto/app/api/model"
	"github.com/opentracing/opentracing-go"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//Create creates a service account for a given space
func Create(ctx context.Context, u *model.User) error {
	log.Debugf("Creating service account '%s'...", u.ID)
	span, ctx := opentracing.StartSpanFromContext(ctx, "k8s.serviceaccounts.crud.create")
	defer span.Finish()

	c := client.Get()
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

//GetCredentials returns the credential for accessing the Okteto Space
func GetCredentials(ctx context.Context, u *model.User, space string) (*model.Credential, error) {
	log.Debug("Get service account credential")
	span, ctx := opentracing.StartSpanFromContext(ctx, "k8s.namespaces.crud.getcredentials")
	defer span.Finish()

	c := client.Get()
	sa, err := c.CoreV1().ServiceAccounts(client.GetOktetoNamespace()).Get(u.ID, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Error getting kubernetes service account: %s", err)
	}
	secret, err := secrets.Get(sa.Secrets[0].Name, client.GetOktetoNamespace(), c)
	if err != nil {
		return nil, err
	}
	cred := &model.Credential{
		Server:      fmt.Sprintf("https://%s", os.Getenv("CLUSTER_PUBLIC_ENDPOINT")),
		Certificate: string(secret.Data["ca.crt"]),
		Token:       string(secret.Data["token"]),
		Namespace:   space,
		Config:      getConfigB64(space, string(secret.Data["ca.crt"]), string(secret.Data["token"])),
	}
	return cred, nil
}

// GetUserByToken gets a user by her token
func GetUserByToken(ctx context.Context, token string) (*model.User, error) {
	if len(token) == 0 {
		return nil, fmt.Errorf("empty token")
	}
	if len(token) > model.TokenLength {
		return nil, fmt.Errorf("malformed token, too long")
	}

	span, ctx := opentracing.StartSpanFromContext(ctx, "k8s.serviceaccounts.crud.getbyuserbytoken")
	defer span.Finish()

	return getByLabel(fmt.Sprintf("%s=%s", OktetoTokenLabel, token))
}

//GetUserByGithubID returns a user by githubID
func GetUserByGithubID(ctx context.Context, githubID string) (*model.User, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "k8s.serviceaccounts.crud.getbyuserbygithubid")
	defer span.Finish()
	return getByLabel(fmt.Sprintf("%s=%s", OktetoGithubIDLabel, githubID))
}

// GetUserByID gets a user by her id
func GetUserByID(ctx context.Context, id string) (*model.User, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "k8s.serviceaccounts.crud.getbyuserbyid")
	defer span.Finish()
	return getByLabel(fmt.Sprintf("%s=%s", OktetoIDLabel, id))
}

//GetUserByEmail returns a user by Email
func GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "k8s.serviceaccounts.crud.getbyuserbytoken")
	defer span.Finish()
	h := hashEmail(strings.ToLower(email))
	return getByLabel(fmt.Sprintf("%s=%s", OktetoEmailLabel, h))
}

func hashEmail(e string) string {
	h := md5.New()
	io.WriteString(h, e)
	return fmt.Sprintf("%x", h.Sum(nil))
}
func getByLabel(label string) (*model.User, error) {
	c := client.Get()
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
	return ToModel(&sas.Items[0]), nil
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
		Invite:   sa.Annotations[OktetoInviteLabel],
	}
}
