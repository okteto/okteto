package main

import (
	"github.com/okteto/app/backend/app"
	"github.com/okteto/app/backend/log"
	"github.com/okteto/app/backend/model"
)

func main() {
	log.Info("Starting app...")
	s := &model.Space{
		Name:    "oktako",
		Members: []string{"oktiko"},
	}
	if err := app.CreateSpace(s); err != nil {
		log.Info("ERROR1", err)
	}

	dev := &model.Dev{
		Name:  "test",
		Image: "okteto/desk:0.1.2",
		WorkDir: &model.Mount{
			Path: "/app",
			Size: "10Gi",
		},
		Command: []string{"sh"},
	}

	if err := app.DevModeOn(dev, s); err != nil {
		log.Info("ERROR2", err)
	}

	log.Info("Exit")
}
