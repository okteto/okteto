package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/google/go-github/github"
	"github.com/okteto/app/api/k8s/users"
	"github.com/okteto/app/api/log"
	"github.com/okteto/app/api/model"

	"golang.org/x/oauth2"
	githubOAuth "golang.org/x/oauth2/github"
)

var oauth2Config = &oauth2.Config{
	ClientID:     os.Getenv("GITHUB_CLIENTID"),
	ClientSecret: os.Getenv("GITHUB_CLIENTSECRET"),
	Endpoint:     githubOAuth.Endpoint,
}

// AuthCLIHandler handles the CLI callback authentication
func AuthCLIHandler() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		state := r.URL.Query().Get("state")
		scheme := "https"
		if r.Host == "localhost" {
			log.Infof("request came from localhost")
			scheme = "http"
		}

		redirectURL, err := url.Parse(fmt.Sprintf("%s://%s/github/callback", scheme, r.Host))
		if err != nil {
			log.Errorf("failed to parse request url: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return

		}

		params := url.Values{}
		params.Add("r", r.URL.Query().Get("redirect"))
		redirectURL.RawQuery = params.Encode()
		config := &oauth2.Config{
			ClientID:     os.Getenv("GITHUB_CLIENTID"),
			ClientSecret: os.Getenv("GITHUB_CLIENTSECRET"),
			Endpoint:     githubOAuth.Endpoint,
			RedirectURL:  redirectURL.String(),
		}

		authURL := config.AuthCodeURL(state)

		http.Redirect(w, r, authURL, http.StatusSeeOther)
		return
	}

	return http.HandlerFunc(fn)
}

// AuthHandler handles the github callback authentication
func AuthHandler() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		redirectURL := r.URL.Query().Get("r")
		e := r.URL.Query().Get("error")
		if len(e) > 0 {
			log.Errorf("github authentication errors: %s", r.URL.Query().Get("error_description"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if len(redirectURL) > 0 {
			// This is the CLI workflow
			cliURL, err := url.Parse(redirectURL)
			if err != nil {
				log.Errorf("malformed redirectURL for the CLI: %s", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			params := url.Values{}
			params.Add("code", r.URL.Query().Get("code"))
			params.Add("state", r.URL.Query().Get("state"))

			cliURL.RawQuery = params.Encode()
			http.Redirect(w, r, cliURL.String(), http.StatusSeeOther)
			return

		}

		// TODO: why is this handler invoked on login from the website?
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

	u := model.NewUser(*githubUser.Login, "", name)
	u, err = users.FindOrCreate(u)
	if err != nil {
		log.Errorf("failed to create user: %s", err)
		return nil, fmt.Errorf("failed to create user")
	}

	return u, nil
}
