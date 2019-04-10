package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/go-github/github"
	"github.com/okteto/app/backend/log"
	"github.com/okteto/app/backend/model"

	"golang.org/x/oauth2"
	githubOAuth "golang.org/x/oauth2/github"
)

var oauth2Config = &oauth2.Config{
	ClientID:     "47867be52b46a2d9d302",
	ClientSecret: "9afa94d61dfac781d18ecc5c49cdfccb61d024a5",
	RedirectURL:  "https://cloud.usa.okteto.net/github/callback",
	Endpoint:     githubOAuth.Endpoint,
}

// AuthHandler handles the github calback authentication
func AuthHandler() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		code := r.FormValue("code")
		if len(code) == 0 {
			log.Errorf("code is missing from request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		u, err := Auth(code)
		if err != nil {
			log.Errorf("github auth failed: %s", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		b, err := json.Marshal(u)
		if err != nil {
			log.Errorf("user marshalling failed: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(b)
		return
	}

	return http.HandlerFunc(fn)
}

// Auth authenticates a github user based in the code
func Auth(code string) (*model.User, error) {
	log.Infof(code)
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

	return model.NewUser(*githubUser.Login, email, name), nil
}
