package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/okteto/app/api/github"
	"github.com/okteto/app/api/graphql"
	"github.com/okteto/app/api/health"
	"github.com/okteto/app/api/log"
)

func main() {
	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "8080"
	}

	validateConfiguration()

	log.Info("Starting app...")
	http.Handle("/healthz", health.HealthcheckHandler())
	http.Handle("/github/callback", github.AuthHandler())
	http.Handle("/github/authorization-code", github.AuthCLIHandler())
	http.Handle("/graphql", graphql.TokenMiddleware(graphql.Handler()))

	log.Infof("Server is running at http://0.0.0.0:%s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
		log.Fatal(err)
	}
}

func validateConfiguration() {
	if len(os.Getenv("CLUSTER_PUBLIC_ENDPOINT")) == 0 {
		log.Fatal("CLUSTER_PUBLIC_ENDPOINT is not defined")
	}

	if len(os.Getenv("OKTETO_CLUSTER_CIDR")) == 0 {
		log.Fatal("OKTETO_CLUSTER_CIDR is not defined")
	}

	if len(os.Getenv("GITHUB_CLIENTID")) == 0 {
		log.Fatal("GITHUB_CLIENTID is not defined")
	}

	if len(os.Getenv("GITHUB_CLIENTSECRET")) == 0 {
		log.Fatal("GITHUB_CLIENTSECRET is not defined")
	}

	if len(os.Getenv("OKTETO_BASE_DOMAIN")) == 0 {
		log.Fatal("OKTETO_BASE_DOMAIN is not defined")
	}
}
