package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/okteto/app/api/github"
	"github.com/okteto/app/api/graphql"
	"github.com/okteto/app/api/health"
	"github.com/okteto/app/api/log"
	"github.com/okteto/app/api/tracing"
	"github.com/opentracing-contrib/go-gorilla/gorilla"
	"github.com/opentracing/opentracing-go"
)

func main() {
	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "8080"
	}

	validateConfiguration()

	tracer, err := tracing.Get()
	if err != nil {
		log.Fatal(err)
	}

	opentracing.SetGlobalTracer(tracer)

	log.Info("Starting app...")
	r := mux.NewRouter()

	r.Handle("/healthz", health.HealthcheckHandler())
	r.Handle("/github/callback", github.AuthHandler())
	r.Handle("/github/authorization-code", github.AuthCLIHandler())
	r.Handle("/graphql", graphql.TokenMiddleware(graphql.Handler()))

	// Add tracing to all routes
	_ = r.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		route.Handler(
			gorilla.Middleware(tracer, route.GetHandler()))
		return nil
	})

	log.Infof("Server is running at http://0.0.0.0:%s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), r); err != nil {
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
