package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/okteto/app/backend/github"
	"github.com/okteto/app/backend/graphql"
	"github.com/okteto/app/backend/log"
)

func main() {
	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "8000"
	}

	log.Info("Starting app...")
	http.Handle("/github/callback", github.AuthHandler())
	http.Handle("/github/authorization-code", github.AuthCLIHandler())
	http.Handle("/graphql", graphql.TokenMiddleware(graphql.Handler()))
	log.Infof("Server is running at http://0.0.0.0:%s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
		log.Fatal(err)
	}
}
