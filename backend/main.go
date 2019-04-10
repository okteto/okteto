package main

import (
	"fmt"
	"net/http"

	"github.com/okteto/app/backend/graphql"
	"github.com/okteto/app/backend/log"
)

func main() {
	log.Info("Starting app...")
	http.Handle("/graphql", graphql.Handler())
	fmt.Println("Server is running on port 8000")
	if err := http.ListenAndServe(":8000", nil); err != nil {
		log.Fatal(err)
	}
}
