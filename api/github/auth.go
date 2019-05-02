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
			Scopes:       []string{"user:email"},
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
	ctx := context.Background()
	log.Info("authenticating user via github")
	t, err := oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, err
	}

	oauthClient := oauth2Config.Client(ctx, t)
	client := github.NewClient(oauthClient)
	githubUser, _, err := client.Users.Get(context.Background(), "")
	if err != nil {
		return nil, err
	}

	log.Info("authenticated user via github")

	if githubUser.Login == nil {
		log.Errorf("githubUser.Login is nil")
		return nil, fmt.Errorf("internal server error")
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

	u := model.NewUser(githubUser.GetLogin(), e, githubUser.GetName(), githubUser.GetAvatarURL())
	u, err = users.FindOrCreate(u)
	if err != nil {
		log.Errorf("failed to create user: %s", err)
		return nil, fmt.Errorf("failed to create user")
	}

	log.Infof("created user via github login: %s", u.ID)

	return u, nil
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
