package cmd

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"math/rand"

	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"
)

var (
	errDone = fmt.Errorf("done")
)

//Login starts the login handshake with github and okteto
func Login() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login [URL]",
		Short: "Login with Okteto",
		RunE: func(cmd *cobra.Command, args []string) error {
			oktetoURL := okteto.CloudURL
			if len(args) > 0 {
				u, err := url.Parse(args[0])
				if err != nil {
					return fmt.Errorf("malformed login URL")
				}

				if u.Scheme == "" {
					u.Scheme = "https"
				}

				oktetoURL = u.String()
			}

			log.Debugf("authenticating with %s", oktetoURL)

			port, err := model.GetAvailablePort()
			if err != nil {
				log.Infof("couldn't access the network: %s", err)
				return fmt.Errorf("couldn't access the network")
			}

			handler := authHandler{
				baseURL:  oktetoURL,
				ctx:      context.Background(),
				state:    randToken(),
				errChan:  make(chan error, 2),
				response: make(chan string, 2),
			}

			go func(h authHandler) {
				http.Handle("/authorization-code/callback", handler.handle())
				h.errChan <- http.ListenAndServe(fmt.Sprintf("127.0.0.1:%d", port), nil)
			}(handler)

			authorizationURL := buildAuthorizationURL(handler.baseURL, handler.state, port)
			fmt.Println("Authentication will continue in your default browser")
			if err := open.Start(authorizationURL); err != nil {
				fmt.Printf("Please open a browser and navigate to the following address:\n")
				fmt.Println(authorizationURL)
			}

			ticker := time.NewTicker(5 * time.Minute)
			var code string
			select {
			case <-ticker.C:
				handler.ctx.Done()
				return fmt.Errorf("authentication timeout")
			case code = <-handler.response:
				break
			case e := <-handler.errChan:
				handler.ctx.Done()
				return e
			}

			fmt.Printf("Received code=%s\n", code)
			fmt.Println("Getting an access token...")
			user, new, err := okteto.Auth(handler.ctx, code, oktetoURL)
			if err != nil {
				return err
			}

			log.Success("Logged in as %s", user)
			analytics.TrackLogin(VersionString, new)
			return nil
		},
	}
	return cmd
}

type authHandler struct {
	ctx      context.Context
	state    string
	baseURL  string
	response chan string
	errChan  chan error
}

func (a *authHandler) handle() http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		s := r.URL.Query().Get("state")

		if a.state != s {
			a.errChan <- fmt.Errorf("Invalid request state")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Write([]byte(`<html><head></head><body onload="window.close();">The authentication will continue in your terminal, you can close this window now.</body></html>`))
		a.response <- code
		return
	}

	return http.HandlerFunc(fn)
}

func randToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

func buildAuthorizationURL(baseURL, state string, port int) string {
	redirectURL := fmt.Sprintf("http://127.0.0.1:%d/authorization-code/callback?state=%s", port, state)
	params := url.Values{}
	params.Add("state", state)
	params.Add("redirect", redirectURL)

	authorizationURL, err := url.Parse(fmt.Sprintf("%s/github/authorization-code", baseURL))
	if err != nil {
		panic(fmt.Sprintf("failed to build authorizationURL: %s", err))
	}
	authorizationURL.RawQuery = params.Encode()
	return authorizationURL.String()
}
