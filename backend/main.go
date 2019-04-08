package main

import (
	"github.com/okteto/app/k8s/client"
	"github.com/okteto/app/log"
	"github.com/okteto/app/model"
)

func main() {
	log.Info("Starting app...")
	s := model.Space{
		Name: "oktako",
		User: "oktiko",
	}
	c := client.Get()
	if err := namespaces.Create(s, c); err != nil 
	{
		log.Info("ERROR": err)
	}

	log.Info("Exit")
}
