package github

import (
	"fmt"
	"net/http"
	"net/url"
	"os"

	"github.com/okteto/app/api/log"
	"golang.org/x/oauth2"
	githubOAuth "golang.org/x/oauth2/github"
)

const (
	inviteKey = "i"
	codeKey   = "code"
)

// AuthCLIHandler handles the CLI callback authentication
func AuthCLIHandler() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		state := r.URL.Query().Get("state")
		redirectURL, err := getRedirectURL(r)
		if err != nil {
			log.Errorf("failed to parse request url: %s", err)
			w.WriteHeader(http.StatusInternalServerError)
			return

		}

		params := url.Values{}
		params.Add("r", r.URL.Query().Get("redirect"))
		redirectURL.RawQuery = params.Encode()

		authURL := getAuthURL(state, redirectURL.String())
		http.Redirect(w, r, authURL, http.StatusSeeOther)
		return
	}

	return http.HandlerFunc(fn)
}

// AuthHandler handles the  callback authentication from github.com for both the CLI and Invitation flows
func AuthHandler() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		e := r.URL.Query().Get("error")
		if len(e) > 0 {
			log.Errorf("github authentication errors: %s", r.URL.Query().Get("error_description"))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		redirectURL := r.URL.Query().Get("r")
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
			http.Redirect(w, r.WithContext(ctx), cliURL.String(), http.StatusSeeOther)
			return

		}

		// TODO: why is this handler invoked on login from the website?
		w.WriteHeader(http.StatusOK)
		return
	}

	return http.HandlerFunc(fn)

}

func getRedirectURL(r *http.Request) (*url.URL, error) {
	scheme := "https"
	if r.Host == "localhost" {
		log.Infof("request came from localhost")
		scheme = "http"
	}

	return url.Parse(fmt.Sprintf("%s://%s/github/callback", scheme, r.Host))
}

func getAuthURL(state, redirectURL string) string {
	config := &oauth2.Config{
		ClientID:     os.Getenv("GITHUB_CLIENTID"),
		ClientSecret: os.Getenv("GITHUB_CLIENTSECRET"),
		Endpoint:     githubOAuth.Endpoint,
		RedirectURL:  redirectURL,
		Scopes:       []string{"user:email"},
	}

	return config.AuthCodeURL(state)
}
