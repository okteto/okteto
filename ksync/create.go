package ksync

import (
	"fmt"

	"github.com/okteto/cnd/model"
	"github.com/vapor-ware/ksync/pkg/ksync"
)

//Create creates a sync folder using ksync
func Create(d *model.Dev, namespace string) error {
	specs := &ksync.SpecList{}
	if err := specs.Update(); err != nil {
		return err
	}
	newSpec := &ksync.SpecDetails{
		Name:          d.Name,
		ContainerName: d.Swap.Deployment.Container,
		Selector:      []string{fmt.Sprintf("cnd=%s", d.Name)},
		Namespace:     namespace,
		LocalPath:     d.Mount.Source,
		RemotePath:    d.Mount.Target,
		Reload:        false,
	}
	if err := newSpec.IsValid(); err != nil {
		return err
	}
	if err := specs.Create(newSpec, true); err != nil {
		return err
	}
	return specs.Save()
}
