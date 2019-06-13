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

		w.Write(loginHTML)
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

var loginHTML = []byte(`
<!DOCTYPE html>
<html>
  <head>
    <meta http-equiv="Content-type" content="text/html; charset=utf-8">
    <title>Okteto is ready!</title>
    <style type="text/css" media="screen">
      body {
        background-color: #ffffff;
        margin: 0;
        padding: 0;
        font-family: "Helvetica Neue", Helvetica, Arial, sans-serif;
      }

      .header {
        margin: 52px;
      }
      
      .header .logo {
        display: block;
        margin: 0 auto;
      }

      .container { 
        margin: 50px auto 40px auto; 
        width: 600px; 
        text-align: center;
      }

      .container .illustration {
        display: block;
        margin: 0 auto;
      }

      h1 {
        font-size: 42px;
        font-weight: 400;
        color: #00D1CA;
        letter-spacing: -1.2px;
        margin-bottom: 0;
      }

      h2 {
        font-size: 24px;
        font-weight: 300;
        color: #5a6071;
        letter-spacing: -0.1px;
        margin: 12px 0;
      }
    </style>
  </head>
  <body>
    <div class="header">
      <svg class="logo" xmlns="http://www.w3.org/2000/svg" width="140" viewBox="0 0 105 29">
        <g fill="none" fillRule="nonzero">
          <path fill="#00D1CA" d="M5.91 2.84c6.4-4.78 15.4-3.37 20.3 3.06.95 1.24.75 3.09-.58 3.92a2.96 2.96 0 0 1-3.95-.64 9.03 9.03 0 0 0-10.03-3.14 8.74 8.74 0 0 0-6 8.5 8.95 8.95 0 0 0 6.2 8.44 8.75 8.75 0 0 0 9.86-3.21c1-1.4 2.47-1.66 3.87-.85 1.4.8 1.52 2.83.65 4.1A14.08 14.08 0 0 1 14.74 29c-4.65.02-9-2.1-11.83-5.81C-2 16.76-.5 7.61 5.9 2.84zm18.97 8.86A2.9 2.9 0 0 1 29 14.5a2.8 2.8 0 0 1-1.59 2.36 2.9 2.9 0 0 1-4.12-2.8 2.8 2.8 0 0 1 1.59-2.36z"/>
          <path fill="#1E222B" d="M60.17 21.58h-3.1l-4.52-6-.04 1.93v4.07h-2.44V3h2.44v11.45l4.14-5.58h3.13v.1l-4.47 5.96 4.86 6.55v.1zm6.4.21a3.77 3.77 0 0 1-2.64-.96 3.52 3.52 0 0 1-1.07-2.67v-7.11h-2.3V8.87h2.3V5.4h2.44v3.46h3.2v2.18h-3.2v6.33c0 .85.17 1.42.5 1.73.34.3.72.45 1.15.45a2.52 2.52 0 0 0 1.09-.23l.77 2.12c-.64.23-1.38.34-2.23.34zm22.27 0a3.77 3.77 0 0 1-2.65-.96 3.52 3.52 0 0 1-1.07-2.67v-7.11h-2.29V8.87h2.29V5.4h2.44v3.46h3.2v2.18h-3.2v6.33c0 .85.17 1.42.5 1.73.34.3.73.45 1.15.45a2.52 2.52 0 0 0 1.1-.23l.76 2.12c-.64.23-1.38.34-2.23.34zm9.25.13a6.83 6.83 0 0 1-6.91-6.75c0-3.72 3.1-6.74 6.91-6.74a6.83 6.83 0 0 1 6.91 6.74c0 3.73-3.1 6.75-6.91 6.75zm0-2.4a4.4 4.4 0 0 0 4.46-4.35c0-2.4-2-4.35-4.46-4.35a4.4 4.4 0 0 0-4.46 4.35c0 2.4 2 4.35 4.46 4.35zm-56.18 2.4A6.83 6.83 0 0 1 35 15.17c0-3.72 3.1-6.74 6.91-6.74a6.83 6.83 0 0 1 6.91 6.74c0 3.73-3.1 6.75-6.91 6.75zm0-2.4a4.4 4.4 0 0 0 4.46-4.35c0-2.4-2-4.35-4.46-4.35a4.4 4.4 0 0 0-4.46 4.35c0 2.4 2 4.35 4.46 4.35zM75.95 22a6.67 6.67 0 0 1-4.9-1.91 6.81 6.81 0 0 1-1.94-4.96c0-1.93.62-3.53 1.87-4.82a6.4 6.4 0 0 1 4.79-1.92c2 0 3.42.64 4.78 1.82 1.36 1.18 2.24 3.4 2.07 5.65H71.7a3.4 3.4 0 0 0 1.33 2.71 4.38 4.38 0 0 0 3.03 1.14c1.61 0 2.96-.8 3.64-2.28h2.66a6.56 6.56 0 0 1-2.75 3.67c-1.08.63-2.3.9-3.65.9zm-4.14-8.33h8.26c-.2-.86-.7-1.65-1.48-2.25-.79-.6-1.48-.9-2.64-.9-.97 0-1.92.34-2.61.9-.7.56-1.3 1.3-1.53 2.25z"/>
        </g>
      </svg>
    </div>
    <div class="container">
      <svg class="illustration" xmlns="http://www.w3.org/2000/svg" width="346" height="244" viewBox="0 0 346 244">
        <g fill="none" fill-rule="evenodd">
          <path fill="#F0FDFD" d="M333 158.584C333 71.002 262.26 0 175 0S17 71.002 17 158.584c0 23.332 5.06 45.466 14.078 65.416h287.844C327.94 204.05 333 181.916 333 158.584"/>
          <path stroke="#1DA8B8" stroke-linecap="round" stroke-linejoin="round" d="M18 224h309M1 224h10M335 224h10"/>
          <path fill="#DAF6F5" d="M255.839 209H94.16C86.341 209 80 202.692 80 194.913V88.087C80 80.307 86.341 74 94.161 74H255.84C263.659 74 270 80.308 270 88.087v106.826c0 7.78-6.341 14.087-14.161 14.087"/>
          <path fill="#FFF" d="M261 74H105.653C91.488 74 80 81.235 80 90.157V188c50.85-15.52 81.818-34.52 92.903-57 11.084-22.48 40.45-41.48 88.097-57z"/>
          <path fill="#00D1CA" fill-rule="nonzero" d="M156.23 111.26c14.345-10.537 34.488-7.434 45.463 6.765 2.114 2.734 1.654 6.812-1.31 8.64-2.966 1.826-6.777 1.28-8.846-1.397-5.284-6.836-14.275-9.632-22.464-6.944-8.184 2.686-13.597 10.246-13.417 18.746.18 8.508 5.543 16.028 13.855 18.635 8.303 2.605 17.097-.191 22.095-7.079 2.229-3.072 5.52-3.653 8.65-1.88 3.13 1.773 3.402 6.26 1.467 9.04-5.991 8.257-15.305 13.164-25.722 13.214-10.41.049-20.17-4.635-26.496-12.823-10.975-14.199-7.619-34.378 6.725-44.916zm42.107 19.328a6.016 6.016 0 0 1 5.988.463 6.087 6.087 0 0 1 2.664 5.4 5.899 5.899 0 0 1-3.326 4.961 6.016 6.016 0 0 1-5.988-.463 6.087 6.087 0 0 1-2.664-5.4 5.899 5.899 0 0 1 3.326-4.961z"/>
          <path fill="#FFF" d="M261 74H105.653C91.488 74 80 81.235 80 90.157V188c50.85-15.52 81.818-34.52 92.903-57 11.084-22.48 40.45-41.48 88.097-57z" opacity=".579"/>
          <path stroke="#1CA8B8" stroke-linecap="round" stroke-linejoin="round" d="M255.839 209H94.16C86.341 209 80 202.692 80 194.913V88.087C80 80.307 86.341 74 94.161 74H255.84C263.659 74 270 80.308 270 88.087v106.826c0 7.78-6.341 14.087-14.161 14.087z"/>
          <path fill="#FFF" d="M191 233h-32l2.667-23h26.666z"/>
          <path stroke="#1CA8B8" stroke-linecap="round" stroke-linejoin="round" d="M191 233h-32l2.667-23h26.666z"/>
          <path fill="#FFF" d="M210 243h-69c0-6.074 4.75-11 10.615-11h47.77c5.861 0 10.615 4.926 10.615 11M253.714 213c8.997 0 16.286-7.725 16.286-17.25V190H80v5.75c0 9.525 7.29 17.25 16.286 17.25h157.428z"/>
          <path stroke="#1CA8B8" stroke-linecap="round" stroke-linejoin="round" d="M253.714 213c8.997 0 16.286-7.725 16.286-17.25V190H80v5.75c0 9.525 7.29 17.25 16.286 17.25h157.428zM210 243h-69c0-6.074 4.75-11 10.615-11h47.77c5.861 0 10.615 4.926 10.615 11z"/>
          <path stroke="#1CA8B8" stroke-linecap="round" stroke-linejoin="round" d="M179 200.5a2.5 2.5 0 1 1-5 0 2.5 2.5 0 0 1 5 0z"/>
          <g>
            <path fill="#FFF" d="M291 85.502C291 105.106 274.877 121 254.998 121 235.118 121 219 105.106 219 85.502 219 65.894 235.118 50 254.998 50S291 65.894 291 85.502"/>
            <path stroke="#1CA8B8" stroke-linecap="round" stroke-linejoin="round" d="M291 85.502C291 105.106 274.877 121 254.998 121 235.118 121 219 105.106 219 85.502 219 65.894 235.118 50 254.998 50S291 65.894 291 85.502z"/>
            <path fill="#BFF0EE" d="M249.411 106a9.238 9.238 0 0 1-6.486-2.633l-12.234-12.005a8.866 8.866 0 0 1 0-12.723c3.583-3.514 9.39-3.514 12.972 0l5.748 5.638 14.925-14.638c3.582-3.519 9.389-3.519 12.977 0a8.88 8.88 0 0 1 0 12.728l-21.415 21A9.23 9.23 0 0 1 249.41 106"/>
            <path stroke="#1CA8B8" stroke-linecap="round" stroke-linejoin="round" d="M249.411 106a9.238 9.238 0 0 1-6.486-2.633l-12.234-12.005a8.866 8.866 0 0 1 0-12.723c3.583-3.514 9.39-3.514 12.972 0l5.748 5.638 14.925-14.638c3.582-3.519 9.389-3.519 12.977 0a8.88 8.88 0 0 1 0 12.728l-21.415 21A9.23 9.23 0 0 1 249.41 106z"/>
          </g>
        </g>
      </svg>
      <h1>You are now logged in!</h1>
      <h2>Your session is now active in the Okteto CLI. You may close this window.</h2>
    </div>
  </body>
</html>
`)
