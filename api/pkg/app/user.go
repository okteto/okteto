package app

import (
	"context"
	"os"
	"strings"

	"github.com/okteto/app/api/pkg/email"
	"github.com/okteto/app/api/pkg/k8s/serviceaccounts"
	"github.com/okteto/app/api/pkg/log"
	"github.com/okteto/app/api/pkg/model"
)

var publicURL = os.Getenv("OKTETO_PUBLIC_URL")

//CreateUser configures a service account for a given user
func CreateUser(ctx context.Context, u *model.User) error {
	if err := serviceaccounts.Create(ctx, u); err != nil {
		return err
	}

	s := model.NewSpace(u.GithubID, u, []model.Member{})
	if err := CreateSpace(s); err != nil {
		return err
	}
	return nil
}

//InviteUser user creates a service account for a given user
func InviteUser(ctx context.Context, email, githubID string) (*model.User, error) {
	u := model.NewUser(githubID, email, "", "")
	u.Invite = model.GetInvite()
	u.Email = email

	if err := serviceaccounts.Create(ctx, u); err != nil {
		return nil, err
	}

	return u, nil
}

//GetCredential returns the credentials of the user for her space
func GetCredential(ctx context.Context, u *model.User, space string) (*model.Credential, error) {
	credential, err := serviceaccounts.GetCredentials(ctx, u, space)
	if err != nil {
		return nil, err
	}

	return credential, err
}

//FindUserByGithubID retrieves user if it exists
func FindUserByGithubID(ctx context.Context, githubID string) (*model.User, error) {
	return serviceaccounts.GetUserByGithubID(ctx, githubID)
}

//FindUserByEmail retrieves user if it exists
func FindUserByEmail(ctx context.Context, email string) (*model.User, error) {
	return serviceaccounts.GetUserByEmail(ctx, email)
}

// IsNotFound returns true if err is of the type not found
func IsNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
}

// InviteNewMembers sends an invite to every new member
func InviteNewMembers(ctx context.Context, sender string, old, new []model.Member) {
	o := make(map[string]struct{})
	for _, m := range old {
		o[m.ID] = struct{}{}
	}

	for _, m := range new {
		if _, ok := o[m.ID]; !ok {
			if err := email.Invite(ctx, publicURL, sender, m.Email); err != nil {
				log.Errorf("failed to send invite email to %s: %s", m.ID, err)
			}
		}
	}
}
