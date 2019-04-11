package github

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/google/go-github/github"
	"github.com/okteto/app/backend/k8s/users"
	"github.com/okteto/app/backend/log"
	"github.com/okteto/app/backend/model"

	"golang.org/x/oauth2"
	githubOAuth "golang.org/x/oauth2/github"
)

var oauth2Config = &oauth2.Config{
	ClientID:     os.Getenv("GITHUB_CLIENTID"),
	ClientSecret: os.Getenv("GITHUB_CLIENTSECRET"),
	Endpoint:     githubOAuth.Endpoint,
}

// AuthHandler handles the github calback authentication
func AuthHandler() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		// TODO: why is this handler invoked on login?
		//code := r.URL.Query().Get("code")
		//log.Infof("auth with code: %s", code)
		//_, err := Auth(code)
		//if err != nil {
		//	log.Errorf("error during authentication: %s", err)
		//	w.WriteHeader(http.StatusUnauthorized)
		//	return
		//}

		w.WriteHeader(http.StatusOK)
		return
	}

	return http.HandlerFunc(fn)
}

// Auth authenticates a github user based in the code
func Auth(code string) (*model.User, error) {
	t, err := oauth2Config.Exchange(oauth2.NoContext, code)
	if err != nil {
		return nil, err
	}

	oauthClient := oauth2Config.Client(oauth2.NoContext, t)
	client := github.NewClient(oauthClient)
	githubUser, _, err := client.Users.Get(context.Background(), "")
	if err != nil {
		return nil, err
	}

	if githubUser.Login == nil {
		return nil, fmt.Errorf("githubUser.Login is nil")
	}

	var name = ""
	if githubUser.Name == nil {
		log.Errorf("githubUser.Name is nil")
	} else {
		name = *githubUser.Name
	}

	email := ""
	if githubUser.Email == nil {
		log.Errorf("githubUser.email is nil")
	} else {
		email = *githubUser.Email
	}

	u := model.NewUser(*githubUser.Login, email, name)
	u, err = users.FindOrCreate(u)
	if err != nil {
		log.Errorf("failed to create user: %s", err)
		return nil, fmt.Errorf("failed to create user")
	}

	return u, nil
}
