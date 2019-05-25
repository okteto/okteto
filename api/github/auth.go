package github

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-github/github"
	"github.com/okteto/app/api/log"

	"golang.org/x/oauth2"
	githubOAuth "golang.org/x/oauth2/github"
)

var oauth2Config = &oauth2.Config{
	ClientID:     os.Getenv("GITHUB_CLIENTID"),
	ClientSecret: os.Getenv("GITHUB_CLIENTSECRET"),
	Endpoint:     githubOAuth.Endpoint,
}

// Auth authenticates a github user based in the code
func Auth(ctx context.Context, code string) (string, string, string, string, error) {
	log.Info("authenticating user via github")
	t, err := oauth2Config.Exchange(ctx, code)
	if err != nil {
		return "", "", "", "", err
	}

	oauthClient := oauth2Config.Client(ctx, t)
	client := github.NewClient(oauthClient)
	githubUser, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return "", "", "", "", err
	}

	log.Info("authenticated user via github")

	if githubUser.Login == nil {
		log.Errorf("githubUser.Login is nil")
		return "", "", "", "", fmt.Errorf("internal server error")
	}

	e := githubUser.GetEmail()
	if len(e) == 0 {
		log.Infof("user doesn't have a visible email displayed, falling back to the github emails API")
		em, err := getUserEmail(ctx, client)
		if err != nil {
			log.Errorf("error when retrieving the email: %s", err)
		} else {
			e = em
		}
	}

	return githubUser.GetLogin(), e, githubUser.GetName(), githubUser.GetAvatarURL(), nil
}

func getUserEmail(ctx context.Context, client *github.Client) (string, error) {
	r, _, err := client.Users.ListEmails(ctx, &github.ListOptions{})
	if err != nil {
		return "", err
	}
	if r != nil {
		for _, em := range r {
			p := em.GetPrimary()
			if p {
				return em.GetEmail(), nil
			}
		}
	}

	return "", fmt.Errorf("not found")
}
