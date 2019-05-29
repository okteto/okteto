package app

import (
	"context"
	"fmt"

	"github.com/okteto/app/api/pkg/k8s/client"
	"github.com/okteto/app/api/pkg/k8s/deployments"
	"github.com/okteto/app/api/pkg/k8s/ingresses"
	"github.com/okteto/app/api/pkg/k8s/secrets"
	"github.com/okteto/app/api/pkg/k8s/services"
	"github.com/okteto/app/api/pkg/k8s/volumes"
	"github.com/okteto/app/api/pkg/model"
	"github.com/opentracing/opentracing-go"
)

//DevModeOn activates a development environment
func DevModeOn(dev *model.Dev, s *model.Space) error {
	if len(dev.Volumes) > 2 {
		return fmt.Errorf("the maximum number of volumes is 2")
	}
	c := client.Get()

	if err := secrets.Create(dev, s, c); err != nil {
		return err
	}

	if err := volumes.Create(dev.GetVolumeName(), s, c); err != nil {
		return err
	}

	for i := range dev.Volumes {
		if err := volumes.Create(dev.GetVolumeDataName(i), s, c); err != nil {
			return err
		}
	}

	if err := deployments.DevOn(dev, s, c); err != nil {
		return err
	}

	new := services.Translate(dev, s)
	if err := services.Deploy(new, s, c); err != nil {
		return err
	}

	if err := ingresses.Deploy(dev, s, c); err != nil {
		return err
	}

	return nil
}

//RunImage runs a docker image
func RunImage(dev *model.Dev, s *model.Space) error {
	c := client.Get()

	if err := deployments.Run(dev, s, c); err != nil {
		return err
	}

	new := services.Translate(dev, s)
	if err := services.Deploy(new, s, c); err != nil {
		return err
	}

	if err := ingresses.Deploy(dev, s, c); err != nil {
		return err
	}

	return nil
}

//DevModeOff deactivates a development environment
func DevModeOff(dev *model.Dev, s *model.Space) error {
	c := client.Get()

	dev = deployments.GetDev(dev, s, c)

	if err := ingresses.Destroy(dev, s, c); err != nil {
		return err
	}

	if err := services.Destroy(dev.Name, s, c); err != nil {
		return err
	}

	if err := deployments.Destroy(dev, s, c); err != nil {
		return err
	}

	if err := volumes.Destroy(dev.GetVolumeName(), s, c); err != nil {
		return err
	}

	for i := range dev.Volumes {
		if err := volumes.Destroy(dev.GetVolumeDataName(i), s, c); err != nil {
			return err
		}
	}

	if err := secrets.Destroy(dev, s, c); err != nil {
		return err
	}

	return nil
}

//ListDevEnvs returns the dev environments for a given user
func ListDevEnvs(ctx context.Context, u *model.User, s *model.Space) ([]*model.Dev, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "app.listdevenvs")
	defer span.Finish()
	c := client.Get()

	deploys, err := deployments.List(s, c)
	if err != nil {
		return nil, fmt.Errorf("error getting deployments: %s", err)
	}

	result := []*model.Dev{}
	for _, d := range deploys {
		dev := &model.Dev{
			ID:    d.Name,
			Space: s.ID,
			Name:  d.Name,
		}
		if deployments.IsDevModeOn(&d) {
			dev.Dev = &model.Member{
				ID:       u.ID,
				Name:     u.Name,
				GithubID: u.GithubID,
				Avatar:   u.Avatar,
				Owner:    true,
			}
		}
		dev.Endpoints = BuildEndpoints(dev, s)
		result = append(result, dev)
	}
	return result, nil
}

// BuildEndpoints builds the endpoints with the FQDN for the specific user and dev env
func BuildEndpoints(dev *model.Dev, s *model.Space) []string {
	return []string{fmt.Sprintf("https://%s", dev.GetEndpoint(s))}
}
