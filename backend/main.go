package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/okteto/app/backend/graphql"
	"github.com/okteto/app/backend/log"
)

func main() {
	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "8000"
	}

	log.Info("Starting app...")
	http.Handle("/graphql", graphql.Handler())
	fmt.Printf("Server is running on port %s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
		log.Fatal(err)
	}
}
